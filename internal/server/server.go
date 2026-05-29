package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/ffprobe"
	"camera/internal/motion"
	"camera/internal/storage"
	"camera/internal/zones"
)

type contextKey int

const claimsKey contextKey = 0

type authClaims struct {
	UserID             int64
	Role               string
	MustChangePassword bool
}

type broadcaster struct {
	mu   sync.Mutex
	subs map[chan motion.Event]struct{}
	done bool
}

func newBroadcaster() *broadcaster {
	return &broadcaster{subs: make(map[chan motion.Event]struct{})}
}

func (b *broadcaster) subscribe() chan motion.Event {
	ch := make(chan motion.Event, 16)
	b.mu.Lock()
	if b.done {
		b.mu.Unlock()
		close(ch)
		return ch
	}
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broadcaster) unsubscribe(ch chan motion.Event) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *broadcaster) run(src <-chan motion.Event) {
	for ev := range src {
		b.mu.Lock()
		for ch := range b.subs {
			select {
			case ch <- ev:
			default:
			}
		}
		b.mu.Unlock()
	}
	b.mu.Lock()
	for ch := range b.subs {
		close(ch)
	}
	b.subs = make(map[chan motion.Event]struct{})
	b.done = true
	b.mu.Unlock()
}

type Server struct {
	cfg                config.ServerConfig
	storageCfg         config.StorageConfig
	logCfg             config.LogConfig
	debug              bool
	timezone           string
	version            string
	commit             string
	builtAt            string
	startTime          time.Time
	cameras            []config.CameraConfig
	log                *slog.Logger
	secret             []byte
	frontend           fs.FS
	mux                *http.ServeMux
	mu                 sync.Mutex
	streamSeen         map[string]time.Time
	motionBroadcasters map[string]*broadcaster
	rawBroadcasters    map[string]*broadcaster
	peakMu             sync.RWMutex
	dailyPeakRaw       map[string]float64
	dailyPeakDate      map[string]string
	snapFn             func(ctx context.Context, rtspURL string) ([]byte, error)
	probedStreams      map[string]ffprobe.StreamInfo
	db                 *db.DB
	prober             *ffprobe.Prober
	onCameraStart      func(config.CameraConfig)
	onCameraStop       func(string)
	monitors           map[string]*motion.Monitor
	cpu                cpuTracker
	cleaner            interface{ ForceClean() }
}

func NewServer(cfg config.ServerConfig, timezone string, cameras []config.CameraConfig, log *slog.Logger, frontend fs.FS) *Server {
	var secret []byte
	if cfg.JWTSecret != "" {
		secret = []byte(cfg.JWTSecret)
	} else {
		secret = make([]byte, 32)
		rand.Read(secret)
	}

	s := &Server{
		cfg:           cfg,
		timezone:      timezone,
		cameras:       cameras,
		log:           log,
		secret:        secret,
		frontend:      frontend,
		mux:           http.NewServeMux(),
		streamSeen:    make(map[string]time.Time),
		probedStreams: make(map[string]ffprobe.StreamInfo),
		startTime:     time.Now(),
	}
	s.routes()
	return s
}

func (s *Server) WithDB(database *db.DB) *Server {
	s.db = database
	return s
}

func (s *Server) WithCameraCallbacks(start func(config.CameraConfig), stop func(string)) *Server {
	s.onCameraStart = start
	s.onCameraStop = stop
	return s
}

func (s *Server) WithStorageConfig(cfg config.StorageConfig) *Server {
	s.storageCfg = cfg
	return s
}

func (s *Server) WithStreamInfo(id string, info ffprobe.StreamInfo) *Server {
	s.probedStreams[id] = info
	return s
}

func (s *Server) WithProber(p *ffprobe.Prober) *Server {
	s.prober = p
	return s
}

func (s *Server) WithVersion(v string) *Server {
	s.version = v
	return s
}

func (s *Server) WithBuildInfo(commit, builtAt string) *Server {
	s.commit = commit
	s.builtAt = builtAt
	return s
}

func (s *Server) WithCleaner(c interface{ ForceClean() }) *Server {
	s.cleaner = c
	return s
}

func (s *Server) WithSystemConfig(debug bool, logCfg config.LogConfig) *Server {
	s.debug = debug
	s.logCfg = logCfg
	return s
}

func (s *Server) WithMotionFeed(cameraID string, events <-chan motion.Event) *Server {
	bc := newBroadcaster()
	s.mu.Lock()
	if s.motionBroadcasters == nil {
		s.motionBroadcasters = make(map[string]*broadcaster)
	}
	s.motionBroadcasters[cameraID] = bc
	s.mu.Unlock()
	go bc.run(events)
	return s
}


func (s *Server) WithSnapshotter(fn func(ctx context.Context, rtspURL string) ([]byte, error)) *Server {
	s.snapFn = fn
	return s
}

func (s *Server) WithMonitor(cameraID string, m *motion.Monitor) *Server {
	s.mu.Lock()
	if s.monitors == nil {
		s.monitors = make(map[string]*motion.Monitor)
	}
	s.monitors[cameraID] = m
	s.mu.Unlock()
	return s
}

func (s *Server) WithRawFeed(cameraID string, events <-chan motion.Event) *Server {
	bc := newBroadcaster()
	s.mu.Lock()
	if s.rawBroadcasters == nil {
		s.rawBroadcasters = make(map[string]*broadcaster)
	}
	s.rawBroadcasters[cameraID] = bc
	s.mu.Unlock()

	tee := make(chan motion.Event, 256)
	go func() {
		defer close(tee)
		for ev := range events {
			s.updateDailyPeak(cameraID, ev)
			tee <- ev
		}
	}()
	go bc.run(tee)
	return s
}

func (s *Server) updateDailyPeak(cameraID string, ev motion.Event) {
	today := ev.Time.UTC().Format("2006-01-02")
	s.peakMu.Lock()
	defer s.peakMu.Unlock()
	if s.dailyPeakRaw == nil {
		s.dailyPeakRaw = make(map[string]float64)
		s.dailyPeakDate = make(map[string]string)
	}
	if s.dailyPeakDate[cameraID] != today {
		s.dailyPeakRaw[cameraID] = ev.Score
		s.dailyPeakDate[cameraID] = today
	} else if ev.Score > s.dailyPeakRaw[cameraID] {
		s.dailyPeakRaw[cameraID] = ev.Score
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/change-password", s.requireAuth(s.handleChangePassword))
	s.mux.HandleFunc("GET /api/config", s.handleClientConfig)
	s.mux.HandleFunc("GET /api/settings", s.requireAdmin(s.handleSettings))
	s.mux.HandleFunc("GET /api/about", s.requireFullAuth(s.handleAbout))
s.mux.HandleFunc("GET /api/cameras", s.requireFullAuth(s.handleCameras))

	s.mux.HandleFunc("GET /api/discover", s.requireAdmin(s.handleDiscover))
	s.mux.HandleFunc("POST /api/discover/streams", s.requireAdmin(s.handleDiscoverStreams))
	s.mux.HandleFunc("GET /api/settings/cameras", s.requireAdmin(s.handleListSettingsCameras))
	s.mux.HandleFunc("POST /api/settings/cameras", s.requireAdmin(s.handleCreateCamera))
	s.mux.HandleFunc("PUT /api/settings/cameras/reorder", s.requireAdmin(s.handleReorderCameras))
	s.mux.HandleFunc("PUT /api/settings/cameras/{id}", s.requireAdmin(s.handleUpdateCamera))
	s.mux.HandleFunc("DELETE /api/settings/cameras/{id}", s.requireAdmin(s.handleDeleteCamera))

	s.mux.HandleFunc("GET /api/users", s.requireAdmin(s.handleListUsers))
	s.mux.HandleFunc("POST /api/users", s.requireAdmin(s.handleCreateUser))
	s.mux.HandleFunc("PUT /api/users/{id}", s.requireAdmin(s.handleUpdateUser))
	s.mux.HandleFunc("DELETE /api/users/{id}", s.requireAdmin(s.handleDeleteUser))

	s.mux.HandleFunc("PUT /api/settings/storage", s.requireAdmin(s.handleUpdateStorageSettings))

	s.mux.HandleFunc("GET /api/drives", s.requireAdmin(s.handleListDrives))
	s.mux.HandleFunc("POST /api/drives", s.requireAdmin(s.handleCreateDrive))
	s.mux.HandleFunc("PUT /api/drives/{id}", s.requireAdmin(s.handleUpdateDrive))
	s.mux.HandleFunc("DELETE /api/drives/{id}", s.requireAdmin(s.handleDeleteDrive))
	s.mux.HandleFunc("GET /api/retention", s.requireAdmin(s.handleListRetentionConfigs))
	s.mux.HandleFunc("PUT /api/retention/{category}", s.requireAdmin(s.handleUpdateRetentionConfig))

	s.mux.HandleFunc("GET /api/settings/analysis", s.requireAdmin(s.handleGetAnalysisConfig))
	s.mux.HandleFunc("PUT /api/settings/analysis", s.requireAdmin(s.handleUpdateAnalysisConfig))
	s.mux.HandleFunc("POST /api/settings/analysis/finetune", s.requireAdmin(s.handleStartFinetune))
	s.mux.HandleFunc("GET /api/settings/analysis/finetune/status/{job_id}", s.requireAdmin(s.handleFinetuneStatus))
	s.mux.HandleFunc("GET /api/settings/cameras/{id}/analysis", s.requireAdmin(s.handleGetCameraAnalysisConfig))
	s.mux.HandleFunc("PUT /api/settings/cameras/{id}/analysis", s.requireAdmin(s.handleUpdateCameraAnalysisConfig))

	streamHandler := http.StripPrefix("/stream/", http.FileServer(http.Dir(s.cfg.SegmentsPath)))
	s.mux.Handle("/stream/", s.requireStreamAccess(streamHandler))

	recHandler := http.StripPrefix("/recordings/", http.FileServer(http.Dir(s.cfg.RecordingsPath)))
	s.mux.Handle("/recordings/", s.requireRecordingsAccess(recHandler))

	s.mux.HandleFunc("GET /api/cameras/{id}/recordings", s.requireCameraAccess(s.handleRecordings))
	s.mux.HandleFunc("GET /api/cameras/{id}/recordings/by-id/{recording_id}", s.requireCameraAccess(s.handleRecordingByID))
	s.mux.HandleFunc("DELETE /api/cameras/{id}/recordings/{filename}", s.requireAdmin(s.handleDeleteRecording))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion", s.requireCameraAccess(s.handleMotionEvents))
	s.mux.HandleFunc("GET /api/motion/live", s.requireFullAuth(s.handleAllMotionLive))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/live", s.requireCameraAccess(s.handleMotionLive))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/scores", s.requireCameraAccess(s.handleMotionScores))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/region-score", s.requireCameraAccess(s.handleMotionRegionScore))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/daily-peak", s.requireCameraAccess(s.handleMotionDailyPeak))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/zones", s.requireCameraAccess(s.handleMotionZonesGet))
	s.mux.HandleFunc("PUT /api/cameras/{id}/motion/zones", s.requireCameraAccess(s.handleMotionZonesPut))
	s.mux.HandleFunc("POST /api/events/{id}/annotations", s.requireFullAuth(s.handleCreateAnnotation))
	s.mux.HandleFunc("GET /api/events/{id}/annotations", s.requireFullAuth(s.handleListAnnotations))
	s.mux.HandleFunc("DELETE /api/annotations/{id}", s.requireFullAuth(s.handleDeleteAnnotation))
	s.mux.HandleFunc("GET /api/settings/analysis/annotation-count", s.requireAdmin(s.handleAnnotationCount))

	s.mux.HandleFunc("GET /api/cameras/{id}/snapshot", s.requireCameraAccess(s.handleSnapshot))
	s.mux.HandleFunc("GET /api/cameras/{id}/stats", s.requireCameraAccess(s.handleCameraStats))
	s.mux.HandleFunc("GET /api/stats", s.requireFullAuth(s.handleStats))

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

// requireFullAuth wraps requireAuth and additionally rejects tokens that have
// must_change_password=true. Only /api/auth/change-password is exempt.
func (s *Server) requireFullAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		ac, _ := r.Context().Value(claimsKey).(authClaims)
		if ac.MustChangePassword {
			http.Error(w, "password change required", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireFullAuth(func(w http.ResponseWriter, r *http.Request) {
		ac, _ := r.Context().Value(claimsKey).(authClaims)
		if ac.Role != "admin" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) canAccessCamera(r *http.Request, cameraID string) bool {
	if s.db == nil {
		return true
	}
	ac, _ := r.Context().Value(claimsKey).(authClaims)
	if ac.Role == "admin" {
		return true
	}
	cameras, err := db.GetUserCameras(s.db, ac.UserID)
	if err != nil {
		return false
	}
	for _, id := range cameras {
		if id == cameraID {
			return true
		}
	}
	return false
}

func (s *Server) requireCameraAccess(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if !s.canAccessCamera(r, r.PathValue("id")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) requireStreamAccess(next http.Handler) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/stream/"), "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		if !s.canAccessCamera(r, parts[0]) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireRecordingsAccess(next http.Handler) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/recordings/"), "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		if !s.canAccessCamera(r, parts[0]) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
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
		parsed, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return s.secret, nil
		})
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
			var ac authClaims
			if uid, ok := claims["user_id"].(float64); ok {
				ac.UserID = int64(uid)
			}
			if roleStr, ok := claims["role"].(string); ok {
				ac.Role = roleStr
			}
			if mcp, ok := claims["must_change_password"].(bool); ok {
				ac.MustChangePassword = mcp
			}
			r = r.WithContext(context.WithValue(r.Context(), claimsKey, ac))
		}
		if strings.HasPrefix(r.URL.Path, "/stream/") {
			s.touchStreamClient(r)
		}
		next(w, r)
	}
}

func (s *Server) touchStreamClient(r *http.Request) {
	key := r.RemoteAddr
	if host := r.Header.Get("X-Forwarded-For"); host != "" {
		key = strings.TrimSpace(strings.Split(host, ",")[0])
	}
	if h, _, ok := strings.Cut(key, ":"); ok && h != "" {
		key = h
	}
	s.mu.Lock()
	s.streamSeen[key] = time.Now()
	s.mu.Unlock()
}

func (s *Server) activeStreamClients(now time.Time) int {
	const activeWindow = 30 * time.Second
	cutoff := now.Add(-activeWindow)
	s.mu.Lock()
	defer s.mu.Unlock()
	active := 0
	for k, seen := range s.streamSeen {
		if seen.Before(cutoff) {
			delete(s.streamSeen, k)
			continue
		}
		active++
	}
	return active
}

func maskRTSP(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	return u.Redacted()
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	type motionDTO struct {
		Enabled             bool    `json:"enabled"`
		Threshold           float64 `json:"threshold"`
		FPS                 int     `json:"fps"`
		CooldownSeconds     int     `json:"cooldown_seconds"`
		CaptureWidth         int `json:"capture_width,omitempty"`
		CaptureHeight        int `json:"capture_height,omitempty"`
		PlaybackLeadSeconds  int `json:"playback_lead_seconds"`
		PlaybackTrailSeconds int `json:"playback_trail_seconds"`
	}
	type cameraDTO struct {
		ID                string     `json:"id"`
		Name              string     `json:"name"`
		RTSPURL           string     `json:"rtsp_url"`
		ChunkDuration     string     `json:"chunk_duration"`
		ReconnectInterval string     `json:"reconnect_interval"`
		VideoCodec        string     `json:"video_codec"`
		HasAudio          *bool      `json:"has_audio"`
		Width             int        `json:"width"`
		Height            int        `json:"height"`
		HLSVideoMode      string     `json:"hls_video_mode"`
		RecordVideoMode   string     `json:"record_video_mode"`
		HLSSegmentSeconds *int       `json:"hls_segment_seconds"`
		HLSListSize       *int       `json:"hls_list_size"`
		HLSDVRSeconds     *int       `json:"hls_dvr_seconds"`
		RecordingEnabled  bool       `json:"recording_enabled"`
		Motion            *motionDTO `json:"motion"`
	}
	camList := s.cameras
	if s.db != nil {
		if all, err := db.ListCameras(s.db); err == nil {
			camList = all
		}
	}
	cameras := make([]cameraDTO, len(camList))
	for i, c := range camList {
		var motion *motionDTO
		if c.Motion != nil {
			motion = &motionDTO{
				Enabled:              c.Motion.Enabled,
				Threshold:            c.Motion.Threshold,
				FPS:                  c.Motion.FPS,
				CooldownSeconds:      c.Motion.CooldownSeconds,
				CaptureWidth:         c.Motion.CaptureWidth,
				CaptureHeight:        c.Motion.CaptureHeight,
				PlaybackLeadSeconds:  c.Motion.PlaybackLeadSeconds,
				PlaybackTrailSeconds: c.Motion.PlaybackTrailSeconds,
			}
		}
		videoCodec := c.VideoCodec
		hasAudio := c.HasAudio
		width, height := c.Width, c.Height
		if probed, ok := s.probedStreams[c.ID]; ok {
			if videoCodec == "" {
				videoCodec = probed.VideoCodec
			}
			if hasAudio == nil {
				ha := probed.HasAudio
				hasAudio = &ha
			}
			if width == 0 {
				width = probed.Width
				height = probed.Height
			}
		}
		cameras[i] = cameraDTO{
			ID:                c.ID,
			Name:              c.Name,
			RTSPURL:           maskRTSP(c.RTSPURL),
			ChunkDuration:     formatDuration(c.EffectiveChunkDuration()),
			ReconnectInterval: formatDuration(c.EffectiveReconnectInterval()),
			VideoCodec:        videoCodec,
			HasAudio:          hasAudio,
			Width:             width,
			Height:            height,
			HLSVideoMode:      c.HLSVideoMode,
			RecordVideoMode:   c.RecordVideoMode,
			HLSSegmentSeconds: c.HLSSegmentSeconds,
			HLSListSize:       c.HLSListSize,
			HLSDVRSeconds:     c.HLSDVRSeconds,
			RecordingEnabled:  c.RecordingEnabled,
			Motion:            motion,
		}
	}
	resp := map[string]any{
		"timezone": s.timezone,
		"debug":    s.debug,
		"log": map[string]any{
			"output": s.logCfg.Output,
			"path":   s.logCfg.Path,
		},
		"server": map[string]any{
			"port":            s.cfg.Port,
			"segments_path":   s.cfg.SegmentsPath,
			"recordings_path": s.cfg.RecordingsPath,
		},
		"storage": func() map[string]any {
			wm, wom, interval, maxGB, warnPct := s.effectiveStorageSettings()
			return map[string]any{
				"path":                   s.storageCfg.Path,
				"with_motion_minutes":    wm,
				"without_motion_minutes": wom,
				"interval_minutes":       interval,
				"max_size_gb":            maxGB,
				"warn_percent":           warnPct,
			}
		}(),
		"defaults": map[string]any{
			"chunk_duration":     formatDuration(config.DefaultChunkDuration),
			"reconnect_interval": formatDuration(config.DefaultReconnectInterval),
		},
		"cameras": cameras,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"version":        s.version,
		"commit":         s.commit,
		"built_at":       s.builtAt,
		"uptime_seconds": time.Since(s.startTime).Seconds(),
		"go_version":     runtime.Version(),
	})
}

func (s *Server) handleClientConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"timezone": s.timezone, "version": s.version})
}

func (s *Server) handleCameras(w http.ResponseWriter, r *http.Request) {
	type motionInfo struct {
		Enabled         bool    `json:"enabled"`
		Threshold       float64 `json:"threshold"`
		FPS             int     `json:"fps"`
		CooldownSeconds int     `json:"cooldown_seconds"`
		CaptureWidth    int     `json:"capture_width,omitempty"`
		CaptureHeight   int     `json:"capture_height,omitempty"`
	}

	type cameraInfo struct {
		ID                   string      `json:"id"`
		Name                 string      `json:"name"`
		RecordingEnabled     bool        `json:"recording_enabled"`
		VideoCodec           string      `json:"video_codec,omitempty"`
		HasAudio             *bool       `json:"has_audio"`
		Width                int         `json:"width,omitempty"`
		Height               int         `json:"height,omitempty"`
		Motion               *motionInfo `json:"motion"`
		MotionThreshold      float64     `json:"motion_threshold"`
		PlaybackLeadSeconds  int         `json:"playback_lead_seconds"`
		PlaybackTrailSeconds int         `json:"playback_trail_seconds"`
	}

	cameras := s.cameras
	if s.db != nil {
		all, err := db.ListCameras(s.db)
		if err == nil {
			cameras = all
		}
		ac, _ := r.Context().Value(claimsKey).(authClaims)
		if ac.Role != "admin" {
			allowed, err := db.GetUserCameras(s.db, ac.UserID)
			if err == nil {
				allowedSet := make(map[string]struct{}, len(allowed))
				for _, id := range allowed {
					allowedSet[id] = struct{}{}
				}
				var filtered []config.CameraConfig
				for _, c := range cameras {
					if _, ok := allowedSet[c.ID]; ok {
						filtered = append(filtered, c)
					}
				}
				cameras = filtered
			}
		}
	}

	list := make([]cameraInfo, len(cameras))
	for i, c := range cameras {
		mc := c.EffectiveMotionConfig()
		lead := 10
		if mc.PlaybackLeadSeconds > 0 {
			lead = mc.PlaybackLeadSeconds
		}
		trail := 10
		if mc.PlaybackTrailSeconds > 0 {
			trail = mc.PlaybackTrailSeconds
		}
		var motion *motionInfo
		if c.Motion != nil {
			motion = &motionInfo{
				Enabled:         mc.Enabled,
				Threshold:       mc.Threshold,
				FPS:             mc.FPS,
				CooldownSeconds: mc.CooldownSeconds,
				CaptureWidth:    mc.CaptureWidth,
				CaptureHeight:   mc.CaptureHeight,
			}
		}
		list[i] = cameraInfo{
			ID:                   c.ID,
			Name:                 c.Name,
			RecordingEnabled:     c.RecordingEnabled,
			VideoCodec:           c.VideoCodec,
			HasAudio:             c.HasAudio,
			Width:                c.Width,
			Height:               c.Height,
			Motion:               motion,
			MotionThreshold:      mc.Threshold,
			PlaybackLeadSeconds:  lead,
			PlaybackTrailSeconds: trail,
		}
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

	type recordingDetection struct {
		Label      string  `json:"label"`
		Confidence float64 `json:"confidence"`
		FrameCount int     `json:"frame_count"`
	}
	type recording struct {
		ID          int64                `json:"id,omitempty"`
		Filename    string               `json:"filename"`
		Start       string               `json:"start"`
		URL         string               `json:"url"`
		IsRecording bool                 `json:"is_recording"`
		HasMotion   bool                 `json:"has_motion"`
		Detections  []recordingDetection `json:"detections,omitempty"`
		mtime       time.Time            // not serialized; used to detect active recording
		path        string               // not serialized; used for DB has_motion lookup
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
				path:     filepath.Join(dir, e.Name()),
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
	// Enrich has_motion and DB id from DB.
	if s.db != nil && len(all) > 0 {
		paths := make([]string, len(all))
		for i, r := range all {
			paths[i] = r.path
		}
		if motionByPath, err := db.HasMotionByPaths(s.db, paths); err == nil {
			for i := range all {
				if motionByPath[all[i].path] {
					all[i].HasMotion = true
				}
			}
		}
		if idsByPath, err := db.IDsByPaths(s.db, paths); err == nil {
			for i := range all {
				all[i].ID = idsByPath[all[i].path]
			}
		}
	}

	// Enrich detections from detections table.
	if s.db != nil && len(all) > 0 {
		paths := make([]string, len(all))
		for i, r := range all {
			paths[i] = r.path
		}
		if detsByPath, err := db.DetectionsByPaths(s.db, paths); err == nil {
			for i := range all {
				if dets := detsByPath[all[i].path]; len(dets) > 0 {
					rd := make([]recordingDetection, len(dets))
					for j, d := range dets {
						rd[j] = recordingDetection{Label: d.Label, Confidence: d.Confidence, FrameCount: d.FrameCount}
					}
					all[i].Detections = rd
				}
			}
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
	json.NewEncoder(w).Encode(map[string]any{"recordings": all[startIdx:endIdx], "hasMore": hasMore, "total": len(all)})
}

func (s *Server) handleRecordingByID(w http.ResponseWriter, r *http.Request) {
	cameraID := r.PathValue("id")
	recIDStr := r.PathValue("recording_id")
	recID, err := strconv.ParseInt(recIDStr, 10, 64)
	if err != nil || recID <= 0 {
		http.Error(w, "invalid recording id", http.StatusBadRequest)
		return
	}
	if s.db == nil {
		http.Error(w, "database not available", http.StatusServiceUnavailable)
		return
	}

	rec, err := db.GetRecordingByID(s.db, recID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if rec.CameraID != cameraID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}
	localStart := rec.StartedAt.In(loc)
	dateStr := localStart.Format("2006-01-02")

	// Derive the URL from the path relative to the recordings root.
	rel, err := filepath.Rel(s.cfg.RecordingsPath, rec.Path)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	url := "/recordings/" + filepath.ToSlash(rel)

	// Check if this is the actively-recording file (mtime < 30s).
	isRecording := false
	if info, err := os.Stat(rec.Path); err == nil {
		isRecording = time.Since(info.ModTime()) < 30*time.Second
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":           rec.ID,
		"filename":     filepath.Base(rec.Path),
		"date":         dateStr,
		"start":        rec.StartedAt.UTC().Format(time.RFC3339),
		"url":          url,
		"is_recording": isRecording,
		"has_motion":   rec.HasMotion,
	})
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

// findRecordingPath returns the filesystem path for filename under cameraID.
// The recorder fixes the output directory at startup time, so a chunk that
// crosses UTC midnight lands in the previous day's directory. We try the day
// derived from the filename (chunkStart) and then the day before.
func findRecordingPath(recordingsPath, cameraID, filename string, chunkStart time.Time) string {
	for _, delta := range []int{0, -1} {
		dir := chunkStart.UTC().AddDate(0, 0, delta).Format("2006/01/02")
		p := filepath.Join(recordingsPath, cameraID, dir, filename)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (s *Server) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	filename := r.PathValue("filename")

	var cam *config.CameraConfig
	for i := range s.cameras {
		if s.cameras[i].ID == id {
			cam = &s.cameras[i]
			break
		}
	}
	if cam == nil {
		http.NotFound(w, r)
		return
	}

	chunkStart, err := storage.ChunkStartFromName(filename)
	if err != nil {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}
	chunkDuration := cam.EffectiveChunkDuration()
	chunkEnd := chunkStart.Add(chunkDuration)

	// The recorder creates the output directory from its startup time, so a
	// chunk that crosses a UTC midnight lands in the previous day's directory.
	// Try the UTC day derived from the filename first, then the day before.
	mp4Path := findRecordingPath(s.cfg.RecordingsPath, id, filename, chunkStart)
	if mp4Path != "" {
		if err := os.Remove(mp4Path); err != nil && !os.IsNotExist(err) {
			http.Error(w, "failed to delete recording", http.StatusInternalServerError)
			return
		}
	}

	if s.db != nil {
		if actualEnd, err := db.EndedAtByStartedAt(s.db, id, chunkStart); err == nil && !actualEnd.IsZero() {
			chunkEnd = actualEnd
		}
		if err := db.DeleteMotionEventsInRange(s.db, id, chunkStart, chunkEnd); err != nil {
			s.log.Warn("failed to clean motion events after recording deletion", "camera", id, "err", err)
		}
		if err := db.DeleteRecordingByStartedAt(s.db, id, chunkStart); err != nil {
			s.log.Warn("failed to remove recording row after deletion", "camera", id, "err", err)
		}
	} else {
		dateDir := chunkStart.UTC().Format("2006/01/02")
		ndjsonPath := filepath.Join(s.cfg.RecordingsPath, id, dateDir, "motion.ndjson")
		if err := storage.RemoveEventsInRange(ndjsonPath, chunkStart, chunkEnd); err != nil {
			s.log.Warn("failed to clean motion events after recording deletion", "path", ndjsonPath, "err", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var recBytes int64
	var recCount int64

	if s.db != nil {
		if c, b, err := db.StatsRecordings(s.db); err == nil {
			recCount, recBytes = c, b
		}
	} else {
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
	}

	var diskTotal, diskFree int64
	if s.cfg.RecordingsPath != "" {
		diskTotal, diskFree = diskStats(s.cfg.RecordingsPath)
	}

	_, _, _, maxGB, warnPct := s.effectiveStorageSettings()
	maxSizeBytes := int64(maxGB * 1024 * 1024 * 1024)

	chunkSec := int64(config.DefaultChunkDuration.Seconds())
	durationSec := recCount * chunkSec

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

	allCameras := s.cameras
	if s.db != nil {
		if cams, err := db.ListCameras(s.db); err == nil {
			allCameras = cams
		}
	}

	type cameraStats struct {
		ID              string     `json:"id"`
		TopMotionScore  float64    `json:"top_motion_score"`
		MinMotionScore  float64    `json:"min_motion_score"`
		Online          bool       `json:"online"`
		LastRecordingAt *time.Time `json:"last_recording_at"`
		MotionEnabled   bool       `json:"motion_enabled"`
	}
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	todayEnd := todayStart.Add(24 * time.Hour)

	var lastRec map[string]time.Time
	if s.db != nil {
		lastRec, _ = db.LastRecordingPerCamera(s.db)
	}

	cameras := make([]cameraStats, len(allCameras))
	onlineThreshold := 5 * time.Minute
	for i, cam := range allCameras {
		mn, mx := motionScoreRange(s.db, s.cfg.RecordingsPath, cam.ID, todayStart, todayEnd)
		cs := cameraStats{
			ID:            cam.ID,
			TopMotionScore: mx,
			MinMotionScore: mn,
			MotionEnabled: cam.EffectiveMotionConfig().Enabled,
		}
		if t, ok := lastRec[cam.ID]; ok {
			ts := t
			cs.LastRecordingAt = &ts
			cs.Online = time.Since(t) <= onlineThreshold
		}
		cameras[i] = cs
	}

	sysMemTotal, sysMemFree := systemMemInfo()
	cpuPct := s.cpu.percent()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"recordings_bytes":            recBytes,
		"recordings_count":            recCount,
		"recordings_duration_seconds": durationSec,
		"forecast_seconds":            forecastSec,
		"disk_total_bytes":            diskTotal,
		"disk_free_bytes":             diskFree,
		"camera_count":                len(allCameras),
		"connected_clients":           s.activeStreamClients(time.Now()),
		"max_size_bytes":              maxSizeBytes,
		"warn_percent":                warnPct,
		"cameras":                     cameras,
		"os":                          osName(),
		"pid":                         os.Getpid(),
		"cpu_percent":                 cpuPct,
		"mem_rss_bytes":               processMemRSS(),
		"sys_mem_total_bytes":         sysMemTotal,
		"sys_mem_free_bytes":          sysMemFree,
		"goroutines":                  runtime.NumGoroutine(),
	})
}

func motionScoreRange(database *db.DB, basePath, cameraID string, start, end time.Time) (min, max float64) {
	if database != nil {
		mn, mx, err := db.MinMaxScoreForDay(database, cameraID, start, end)
		if err == nil {
			return mn, mx
		}
	}
	utcDay := start.UTC().Format("2006/01/02")
	path := filepath.Join(basePath, cameraID, utcDay, "motion.ndjson")
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	first := true
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev struct {
			Score float64 `json:"score"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if first || ev.Score < min {
			min = ev.Score
		}
		if first || ev.Score > max {
			max = ev.Score
		}
		first = false
	}
	return min, max
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

	if s.db == nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	user, err := db.GetUserByUsername(s.db, creds.Username)
	if err != nil || !db.CheckPassword(user.PasswordHash, creds.Password) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":                  creds.Username,
		"exp":                  time.Now().Add(24 * time.Hour).Unix(),
		"user_id":              user.ID,
		"role":                 user.Role,
		"must_change_password": user.MustChangePassword,
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": signed})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	ac, _ := r.Context().Value(claimsKey).(authClaims)

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := db.ClearMustChangePassword(s.db, ac.UserID, body.Password); err != nil {
		s.log.Error("change password failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMotionLive(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	bc := s.motionBroadcasters[id]
	s.mu.Unlock()
	if bc == nil {
		http.NotFound(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := bc.subscribe()
	defer bc.unsubscribe(sub)

	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				return
			}
			payload := map[string]any{
				"time":  ev.Time.Format(time.RFC3339),
				"score": ev.Score,
			}
			if ev.Label != "" {
				payload["label"] = ev.Label
			}
			if ev.Color != "" {
				payload["color"] = ev.Color
			}
			data, _ := json.Marshal(payload)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleAllMotionLive(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	type entry struct {
		id   string
		name string
		bc   *broadcaster
	}
	cameraNames := make(map[string]string, len(s.cameras))
	for _, c := range s.cameras {
		cameraNames[c.ID] = c.Name
	}
	var entries []entry
	for id, bc := range s.motionBroadcasters {
		if s.canAccessCamera(r, id) {
			entries = append(entries, entry{id, cameraNames[id], bc})
		}
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if len(entries) == 0 {
		<-r.Context().Done()
		return
	}

	type taggedEvent struct {
		cameraID   string
		cameraName string
		ev         motion.Event
	}
	merged := make(chan taggedEvent, 64)

	var wg sync.WaitGroup
	for _, e := range entries {
		camID := e.id
		camName := e.name
		bc := e.bc
		sub := bc.subscribe()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer bc.unsubscribe(sub)
			for {
				select {
				case ev, ok := <-sub:
					if !ok {
						return
					}
					select {
					case merged <- taggedEvent{cameraID: camID, cameraName: camName, ev: ev}:
					case <-r.Context().Done():
						return
					}
				case <-r.Context().Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(merged)
	}()

	for {
		select {
		case te, ok := <-merged:
			if !ok {
				return
			}
			payload := map[string]any{
				"camera_id":   te.cameraID,
				"camera_name": te.cameraName,
				"time":        te.ev.Time.Format(time.RFC3339),
				"score":       te.ev.Score,
			}
			if te.ev.Label != "" {
				payload["label"] = te.ev.Label
			}
			if te.ev.Color != "" {
				payload["color"] = te.ev.Color
			}
			data, _ := json.Marshal(payload)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleMotionScores(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	bc := s.rawBroadcasters[id]
	s.mu.Unlock()
	if bc == nil {
		http.NotFound(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sub := bc.subscribe()
	defer bc.unsubscribe(sub)

	for {
		select {
		case ev, ok := <-sub:
			if !ok {
				return
			}
			data, _ := json.Marshal(map[string]any{
				"time":  ev.Time.Format(time.RFC3339),
				"score": ev.Score,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleMotionRegionScore(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	parseF := func(key string) (float64, bool) {
		v := r.URL.Query().Get(key)
		if v == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	}
	x, okX := parseF("x")
	y, okY := parseF("y")
	wf, okW := parseF("w")
	hf, okH := parseF("h")
	if !okX || !okY || !okW || !okH {
		http.Error(w, "x, y, w, h query params required", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	mon := s.monitors[id]
	s.mu.Unlock()
	if mon == nil {
		http.NotFound(w, r)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	bbox := motion.BBox{X: x, Y: y, W: wf, H: hf}
	inspID, ch := mon.RegisterInspector(bbox)
	defer mon.UnregisterInspector(inspID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case score, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(map[string]any{"score": score})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) handleMotionDailyPeak(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	_, hasRaw := s.rawBroadcasters[id]
	s.mu.Unlock()
	if !hasRaw {
		http.NotFound(w, r)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	s.peakMu.RLock()
	peak := s.dailyPeakRaw[id]
	date := s.dailyPeakDate[id]
	s.peakMu.RUnlock()

	if date != today {
		peak = 0
		date = today
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"camera_id":      id,
		"peak_raw_score": peak,
		"date":           date,
	})
}

func (s *Server) cameraExists(id string) bool {
	for _, c := range s.cameras {
		if c.ID == id {
			return true
		}
	}
	return false
}

func (s *Server) cameraRTSP(id string) string {
	for _, c := range s.cameras {
		if c.ID == id {
			return c.RTSPURL
		}
	}
	return ""
}

func (s *Server) handleMotionZonesGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	var zs []zones.Zone
	if s.db != nil {
		var err error
		zs, err = db.GetZones(s.db, id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	} else {
		zs = []zones.Zone{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(zs)
}

func (s *Server) handleMotionZonesPut(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	var zs []zones.Zone
	if err := json.NewDecoder(r.Body).Decode(&zs); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	for _, z := range zs {
		if z.X < 0 || z.Y < 0 || z.W <= 0 || z.H <= 0 ||
			z.X > 1 || z.Y > 1 || z.X+z.W > 1 || z.Y+z.H > 1 {
			http.Error(w, "zona inválida: coordenadas fora do intervalo [0,1]", http.StatusBadRequest)
			return
		}
	}
	if s.db == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := db.SetZones(s.db, id, zs); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rtsp := s.cameraRTSP(id)
	if rtsp == "" {
		http.NotFound(w, r)
		return
	}
	if s.snapFn == nil {
		http.Error(w, "snapshot not available", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	data, err := s.snapFn(ctx, rtsp)
	if err != nil || len(data) == 0 {
		http.Error(w, "snapshot failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(data)
}

func (s *Server) handleMotionEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dateStr := r.URL.Query().Get("date")

	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}
	localDay, err := time.ParseInLocation("2006-01-02", dateStr, loc)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	dayStart := localDay.UTC()
	dayEnd := localDay.Add(24 * time.Hour).UTC()

	var events []map[string]any
	if s.db != nil {
		rows, err := db.ListMotionEvents(s.db, id, dayStart, dayEnd)
		if err == nil {
			for _, ev := range rows {
				entry := map[string]any{
					"id":    ev.ID,
					"time":  ev.OccurredAt.UTC().Format(time.RFC3339),
					"score": ev.Score,
					"bbox":  map[string]float64{"x": ev.BboxX, "y": ev.BboxY, "w": ev.BboxW, "h": ev.BboxH},
				}
				if ev.FramePath != "" {
					entry["frame"] = ev.FramePath
				}
				if ev.Label != "" {
					entry["label"] = ev.Label
				}
				if ev.Color != "" {
					entry["color"] = ev.Color
				}
				events = append(events, entry)
			}
		}
	} else {
		utcDays := utcDaysInRange(dayStart, dayEnd)
		for _, utcDay := range utcDays {
			ndjsonPath := filepath.Join(s.cfg.RecordingsPath, id, utcDay.Format("2006/01/02"), "motion.ndjson")
			f, err := os.Open(ndjsonPath)
			if err != nil {
				continue
			}
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				var ev map[string]any
				if json.Unmarshal(sc.Bytes(), &ev) != nil {
					continue
				}
				if timeStr, ok := ev["time"].(string); ok {
					t, err := time.Parse(time.RFC3339, timeStr)
					if err != nil || t.Before(dayStart) || !t.Before(dayEnd) {
						continue
					}
				}
				events = append(events, ev)
			}
			f.Close()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if events == nil {
		json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"events": events})
}
