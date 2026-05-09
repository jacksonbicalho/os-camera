package server

import (
	"bufio"
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
	"camera/internal/motion"
)

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
	defaults           config.DefaultsConfig
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
	motionCfg          config.MotionConfig
	peakMu             sync.RWMutex
	dailyPeakRaw       map[string]float64
	dailyPeakDate      map[string]string
}

func NewServer(cfg config.ServerConfig, timezone string, cameras []config.CameraConfig, log *slog.Logger, frontend fs.FS) *Server {
	secret := make([]byte, 32)
	rand.Read(secret)

	s := &Server{
		cfg:        cfg,
		timezone:   timezone,
		cameras:    cameras,
		log:        log,
		secret:     secret,
		frontend:   frontend,
		mux:        http.NewServeMux(),
		streamSeen: make(map[string]time.Time),
		startTime:  time.Now(),
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

func (s *Server) WithVersion(v string) *Server {
	s.version = v
	return s
}

func (s *Server) WithBuildInfo(commit, builtAt string) *Server {
	s.commit = commit
	s.builtAt = builtAt
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

func (s *Server) WithMotionConfig(cfg config.MotionConfig) *Server {
	s.motionCfg = cfg
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
	s.mux.HandleFunc("GET /api/config", s.handleClientConfig)
	s.mux.HandleFunc("GET /api/settings", s.requireAuth(s.handleSettings))
	s.mux.HandleFunc("GET /api/about", s.requireAuth(s.handleAbout))
	s.mux.HandleFunc("GET /api/cameras", s.requireAuth(s.handleCameras))

	streamHandler := http.StripPrefix("/stream/", http.FileServer(http.Dir(s.cfg.SegmentsPath)))
	s.mux.Handle("/stream/", s.requireAuth(streamHandler.ServeHTTP))

	recHandler := http.StripPrefix("/recordings/", http.FileServer(http.Dir(s.cfg.RecordingsPath)))
	s.mux.Handle("/recordings/", s.requireAuth(recHandler.ServeHTTP))

	s.mux.HandleFunc("GET /api/cameras/{id}/recordings", s.requireAuth(s.handleRecordings))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion", s.requireAuth(s.handleMotionEvents))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/live", s.requireAuth(s.handleMotionLive))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/scores", s.requireAuth(s.handleMotionScores))
	s.mux.HandleFunc("GET /api/cameras/{id}/motion/daily-peak", s.requireAuth(s.handleMotionDailyPeak))
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
		Enabled         bool    `json:"enabled"`
		Threshold       float64 `json:"threshold"`
		FPS             int     `json:"fps"`
		CooldownSeconds int     `json:"cooldown_seconds"`
	}
	type cameraDTO struct {
		ID                string     `json:"id"`
		RTSPURL           string     `json:"rtsp_url"`
		ChunkDuration     string     `json:"chunk_duration"`
		ReconnectInterval string     `json:"reconnect_interval"`
		VideoCodec        string     `json:"video_codec"`
		HasAudio          *bool      `json:"has_audio"`
		Width             int        `json:"width"`
		Height            int        `json:"height"`
		Motion            *motionDTO `json:"motion"`
	}
	cameras := make([]cameraDTO, len(s.cameras))
	for i, c := range s.cameras {
		var motion *motionDTO
		if c.Motion != nil {
			motion = &motionDTO{
				Enabled:         c.Motion.Enabled,
				Threshold:       c.Motion.Threshold,
				FPS:             c.Motion.FPS,
				CooldownSeconds: c.Motion.CooldownSeconds,
			}
		}
		cameras[i] = cameraDTO{
			ID:                c.ID,
			RTSPURL:           maskRTSP(c.RTSPURL),
			ChunkDuration:     time.Duration(c.ChunkDuration).String(),
			ReconnectInterval: time.Duration(c.ReconnectInterval).String(),
			VideoCodec:        c.VideoCodec,
			HasAudio:          c.HasAudio,
			Width:             c.Width,
			Height:            c.Height,
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
			"hls_dvr_seconds": s.cfg.HLSDVRSeconds,
			"username":        s.cfg.Username,
		},
		"storage": map[string]any{
			"path":              s.storageCfg.Path,
			"retention_minutes": s.storageCfg.RetentionMinutes,
			"interval_minutes":  s.storageCfg.IntervalMinutes,
			"max_size_gb":       s.storageCfg.MaxSizeGB,
			"warn_percent":      s.storageCfg.WarnPercent,
		},
		"motion": motionDTO{
			Enabled:         s.motionCfg.Enabled,
			Threshold:       s.motionCfg.Threshold,
			FPS:             s.motionCfg.FPS,
			CooldownSeconds: s.motionCfg.CooldownSeconds,
		},
		"defaults": map[string]any{
			"chunk_duration":     time.Duration(s.defaults.ChunkDuration).String(),
			"reconnect_interval": time.Duration(s.defaults.ReconnectInterval).String(),
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
	type cameraInfo struct {
		ID              string  `json:"id"`
		MotionThreshold float64 `json:"motion_threshold"`
	}
	list := make([]cameraInfo, len(s.cameras))
	for i, c := range s.cameras {
		list[i] = cameraInfo{
			ID:              c.ID,
			MotionThreshold: c.EffectiveMotionConfig(s.motionCfg).Threshold,
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
		diskTotal, diskFree = diskStats(s.cfg.RecordingsPath)
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

	type cameraStats struct {
		ID             string  `json:"id"`
		TopMotionScore float64 `json:"top_motion_score"`
		MinMotionScore float64 `json:"min_motion_score"`
	}
	today := time.Now().UTC().Format("2006/01/02")
	cameras := make([]cameraStats, len(s.cameras))
	for i, cam := range s.cameras {
		mn, mx := motionScoreRange(s.cfg.RecordingsPath, cam.ID, today)
		cameras[i] = cameraStats{ID: cam.ID, TopMotionScore: mx, MinMotionScore: mn}
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
		"connected_clients":           s.activeStreamClients(time.Now()),
		"max_size_bytes":              maxSizeBytes,
		"warn_percent":                s.storageCfg.WarnPercent,
		"cameras":                     cameras,
	})
}

func motionScoreRange(basePath, cameraID, utcDay string) (min, max float64) {
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
	utcDays := utcDaysInRange(dayStart, dayEnd)

	var events []map[string]any
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

	w.Header().Set("Content-Type", "application/json")
	if events == nil {
		json.NewEncoder(w).Encode(map[string]any{"events": []any{}})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"events": events})
}
