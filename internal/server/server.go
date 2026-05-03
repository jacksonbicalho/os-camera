package server

import (
	"crypto/rand"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"camera/internal/config"
)

type Server struct {
	cfg      config.ServerConfig
	cameras  []config.CameraConfig
	log      *slog.Logger
	secret   []byte
	frontend fs.FS
	mux      *http.ServeMux
}

func NewServer(cfg config.ServerConfig, cameras []config.CameraConfig, log *slog.Logger, frontend fs.FS) *Server {
	secret := make([]byte, 32)
	rand.Read(secret)

	s := &Server{
		cfg:      cfg,
		cameras:  cameras,
		log:      log,
		secret:   secret,
		frontend: frontend,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("GET /api/config", s.handleClientConfig)
	s.mux.HandleFunc("GET /api/cameras", s.requireAuth(s.handleCameras))

	streamHandler := http.StripPrefix("/stream/", http.FileServer(http.Dir(s.cfg.SegmentsPath)))
	s.mux.Handle("/stream/", s.requireAuth(streamHandler.ServeHTTP))

	recHandler := http.StripPrefix("/recordings/", http.FileServer(http.Dir(s.cfg.RecordingsPath)))
	s.mux.Handle("/recordings/", s.requireAuth(recHandler.ServeHTTP))

	s.mux.HandleFunc("GET /api/cameras/{id}/recordings", s.requireAuth(s.handleRecordings))

	if s.frontend != nil {
		s.mux.Handle("/", s.spaHandler())
	}
}

func (s *Server) spaHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.frontend))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading "/" to form a valid fs.FS path
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(s.frontend, name); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Unknown path: serve index.html for client-side routing
		data, err := fs.ReadFile(s.frontend, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""
		if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
			tokenStr = h[7:]
		} else if q := r.URL.Query().Get("token"); q != "" {
			tokenStr = q
		}
		if tokenStr == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return s.secret, nil
		})
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleClientConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"timezone": s.cfg.Timezone})
}

func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	type cameraInfo struct {
		ID string `json:"id"`
	}
	list := make([]cameraInfo, len(s.cameras))
	for i, c := range s.cameras {
		list[i] = cameraInfo{ID: c.ID}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleRecordings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dateStr := r.URL.Query().Get("date")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	dir := filepath.Join(s.cfg.RecordingsPath, id, t.Format("2006/01/02"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"recordings": []any{}, "hasMore": false})
		return
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".mp4") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	start := (page - 1) * limit
	if start >= len(files) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"recordings": []any{}, "hasMore": false})
		return
	}
	end := start + limit
	hasMore := end < len(files)
	if end > len(files) {
		end = len(files)
	}

	type recording struct {
		Filename string `json:"filename"`
		Start    string `json:"start"`
		URL      string `json:"url"`
	}
	result := make([]recording, 0, end-start)
	for _, name := range files[start:end] {
		ts, _ := time.ParseInLocation("20060102150405", strings.TrimSuffix(name, ".mp4"), time.UTC)
		result = append(result, recording{
			Filename: name,
			Start:    ts.UTC().Format(time.RFC3339),
			URL:      "/recordings/" + id + "/" + t.Format("2006/01/02") + "/" + name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"recordings": result, "hasMore": hasMore})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if creds.Username != s.cfg.Username || creds.Password != s.cfg.Password {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": creds.Username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": signed})
}
