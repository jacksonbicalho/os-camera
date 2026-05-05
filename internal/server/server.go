package server

import (
	"bufio"
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
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"camera/internal/config"
)

type Server struct {
	cfg        config.ServerConfig
	storageCfg config.StorageConfig
	defaults   config.DefaultsConfig
	timezone   string
	cameras    []config.CameraConfig
	log        *slog.Logger
	secret     []byte
	frontend   fs.FS
	mux        *http.ServeMux
}

func NewServer(cfg config.ServerConfig, timezone string, cameras []config.CameraConfig, log *slog.Logger, frontend fs.FS) *Server {
	secret := make([]byte, 32)
	rand.Read(secret)

	s := &Server{
		cfg:      cfg,
		timezone: timezone,
		cameras:  cameras,
		log:      log,
		secret:   secret,
		frontend: frontend,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) WithStorageConfig(cfg config.StorageConfig) *Server {
	s.storageCfg = cfg
	return s
}

func (s *Server) WithDefaults(cfg config.DefaultsConfig) *Server {
	s.defaults = cfg
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
	s.mux.HandleFunc("GET /api/cameras/{id}/motion", s.requireAuth(s.handleMotionEvents))
	s.mux.HandleFunc("GET /api/stats", s.requireAuth(s.handleStats))

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
	json.NewEncoder(w).Encode(map[string]string{"timezone": s.timezone})
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
	order := r.URL.Query().Get("order")
	if order != "asc" {
		order = "desc"
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}

	// Parse the requested date as a local day in the configured timezone.
	localDay, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	// UTC range that covers the full local day.
	dayStart := localDay.UTC()
	dayEnd := localDay.Add(24 * time.Hour).UTC()

	// Collect UTC calendar days that overlap with this local day.
	utcDays := utcDaysInRange(dayStart, dayEnd)

	type recording struct {
		Filename    string    `json:"filename"`
		Start       string    `json:"start"`
		URL         string    `json:"url"`
		IsRecording bool      `json:"is_recording"`
		mtime       time.Time // not serialized; used to detect active recording
	}

	var all []recording
	for _, utcDay := range utcDays {
		dir := filepath.Join(s.cfg.RecordingsPath, id, utcDay.Format("2006/01/02"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".mp4") {
				continue
			}
			ts, err := time.ParseInLocation("20060102150405", strings.TrimSuffix(e.Name(), ".mp4"), time.UTC)
			if err != nil {
				continue
			}
			if ts.Before(dayStart) || !ts.Before(dayEnd) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			all = append(all, recording{
				Filename: e.Name(),
				Start:    ts.UTC().Format(time.RFC3339),
				URL:      "/recordings/" + id + "/" + utcDay.Format("2006/01/02") + "/" + e.Name(),
				mtime:    info.ModTime(),
			})
		}
	}

	// Only the file with the latest filename (= latest segment start) can be
	// actively recording. Marking all recent-mtime files would show two "REC"
	// badges during the brief overlap when a chunk closes and a new one opens.
	if len(all) > 0 {
		latest := 0
		for i := range all {
			if all[i].Filename > all[latest].Filename {
				latest = i
			}
		}
		if time.Since(all[latest].mtime) < 30*time.Second {
			all[latest].IsRecording = true
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if order == "asc" {
			return all[i].Filename < all[j].Filename
		}
		return all[i].Filename > all[j].Filename
	})

	empty := map[string]any{"recordings": []any{}, "hasMore": false}
	startIdx := (page - 1) * limit
	if startIdx >= len(all) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(empty)
		return
	}
	endIdx := startIdx + limit
	hasMore := endIdx < len(all)
	if endIdx > len(all) {
		endIdx = len(all)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"recordings": all[startIdx:endIdx], "hasMore": hasMore})
}

// utcDaysInRange returns the distinct UTC calendar days that overlap [start, end).
func utcDaysInRange(start, end time.Time) []time.Time {
	var days []time.Time
	d := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	for d.Before(end) {
		days = append(days, d)
		d = d.AddDate(0, 0, 1)
	}
	return days
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var recBytes int64
	var recCount int
	filepath.WalkDir(s.cfg.RecordingsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".mp4" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		recBytes += info.Size()
		recCount++
		return nil
	})

	var diskTotal, diskFree int64
	if s.cfg.RecordingsPath != "" {
		var stat syscall.Statfs_t
			if err := syscall.Statfs(s.cfg.RecordingsPath, &stat); err == nil {
				diskTotal = int64(stat.Blocks) * int64(stat.Frsize)
				diskFree = int64(stat.Bavail) * int64(stat.Frsize)
			}
		}

	maxSizeBytes := int64(s.storageCfg.MaxSizeGB * 1024 * 1024 * 1024)

	chunkSec := int64(time.Duration(s.defaults.ChunkDuration).Seconds())
	if chunkSec <= 0 {
		chunkSec = 300 // default 5 min
	}
	durationSec := int64(recCount) * chunkSec

	availableBytes := diskFree
	if maxSizeBytes > 0 {
		availableBytes = max(0, maxSizeBytes-recBytes)
	}
	// forecast = availableBytes * durationSec / recBytes
	// (avoids integer division truncating bytes_per_sec to 0 for small files)
	var forecastSec int64
	if durationSec > 0 && recBytes > 0 {
		forecastSec = availableBytes * durationSec / recBytes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"recordings_bytes":            recBytes,
		"recordings_count":            recCount,
		"recordings_duration_seconds": durationSec,
		"forecast_seconds":            forecastSec,
		"disk_total_bytes":            diskTotal,
		"disk_free_bytes":             diskFree,
		"camera_count":                len(s.cameras),
		"max_size_bytes":              maxSizeBytes,
		"warn_percent":                s.storageCfg.WarnPercent,
	})
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

func (s *Server) handleMotionEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dateStr := r.URL.Query().Get("date")

	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}

	// dateStr is YYYY-MM-DD; convert slashes for directory path
	datePath := strings.ReplaceAll(dateStr, "-", "/")
	ndjsonPath := filepath.Join(s.cfg.RecordingsPath, id, datePath, "motion.ndjson")

	empty := map[string]any{"events": []any{}}

	f, err := os.Open(ndjsonPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(empty)
		return
	}
	defer f.Close()

	var events []map[string]any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev map[string]any
		if json.Unmarshal(sc.Bytes(), &ev) == nil {
			events = append(events, ev)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"events": events})
}
