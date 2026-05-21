package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/ffprobe"
)

func validateMotionConfig(m *motionConfigDTO) error {
	if m.Threshold != 0 && (m.Threshold < 0.001 || m.Threshold > 1.0) {
		return fmt.Errorf("threshold must be between 0.001 and 1.0 (got %.4f)", m.Threshold)
	}
	if m.FPS != 0 && (m.FPS < 1 || m.FPS > 30) {
		return fmt.Errorf("fps must be between 1 and 30 (got %d)", m.FPS)
	}
	if m.CooldownSeconds < 0 {
		return fmt.Errorf("cooldown_seconds must be >= 0 (got %d)", m.CooldownSeconds)
	}
	if m.PlaybackLeadSeconds < 0 || m.PlaybackLeadSeconds > 300 {
		return fmt.Errorf("playback_lead_seconds must be between 0 and 300 (got %d)", m.PlaybackLeadSeconds)
	}
	if m.CaptureWidth < 0 || m.CaptureHeight < 0 {
		return fmt.Errorf("capture dimensions must be >= 0")
	}
	return nil
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	total := int64(d)
	if total%int64(time.Hour) == 0 {
		return fmt.Sprintf("%dh", total/int64(time.Hour))
	}
	if total%int64(time.Minute) == 0 {
		return fmt.Sprintf("%dm", total/int64(time.Minute))
	}
	if total%int64(time.Second) == 0 {
		return fmt.Sprintf("%ds", total/int64(time.Second))
	}
	return d.String()
}

type motionConfigDTO struct {
	Enabled             bool    `json:"enabled"`
	Threshold           float64 `json:"threshold"`
	FPS                 int     `json:"fps"`
	CooldownSeconds     int     `json:"cooldown_seconds"`
	CaptureWidth        int     `json:"capture_width,omitempty"`
	CaptureHeight       int     `json:"capture_height,omitempty"`
	PlaybackLeadSeconds int     `json:"playback_lead_seconds"`
}

type cameraConfigDTO struct {
	ID                string           `json:"id"`
	RTSPURL           string           `json:"rtsp_url"`
	ChunkDuration     string           `json:"chunk_duration"`
	ReconnectInterval string           `json:"reconnect_interval"`
	VideoCodec        string           `json:"video_codec,omitempty"`
	HasAudio          *bool            `json:"has_audio"`
	Width             int              `json:"width,omitempty"`
	Height            int              `json:"height,omitempty"`
	DisplayOrder      int              `json:"display_order"`
	HLSVideoMode      string           `json:"hls_video_mode"`
	RecordVideoMode   string           `json:"record_video_mode"`
	HLSSegmentSeconds *int             `json:"hls_segment_seconds"`
	HLSListSize       *int             `json:"hls_list_size"`
	Motion            *motionConfigDTO `json:"motion"`
}

func cameraToDTO(cam config.CameraConfig) cameraConfigDTO {
	dto := cameraConfigDTO{
		ID:                cam.ID,
		RTSPURL:           cam.RTSPURL,
		ChunkDuration:     formatDuration(cam.EffectiveChunkDuration()),
		ReconnectInterval: formatDuration(cam.EffectiveReconnectInterval()),
		VideoCodec:        cam.VideoCodec,
		HasAudio:          cam.HasAudio,
		Width:             cam.Width,
		Height:            cam.Height,
		DisplayOrder:      cam.DisplayOrder,
		HLSVideoMode:      cam.HLSVideoMode,
		RecordVideoMode:   cam.RecordVideoMode,
		HLSSegmentSeconds: cam.HLSSegmentSeconds,
		HLSListSize:       cam.HLSListSize,
	}
	if cam.Motion != nil {
		dto.Motion = &motionConfigDTO{
			Enabled:             cam.Motion.Enabled,
			Threshold:           cam.Motion.Threshold,
			FPS:                 cam.Motion.FPS,
			CooldownSeconds:     cam.Motion.CooldownSeconds,
			CaptureWidth:        cam.Motion.CaptureWidth,
			CaptureHeight:       cam.Motion.CaptureHeight,
			PlaybackLeadSeconds: cam.Motion.PlaybackLeadSeconds,
		}
	}
	return dto
}

// reloadCamerasFromDB replaces s.cameras with the current DB state.
func (s *Server) reloadCamerasFromDB() {
	cams, err := db.ListCameras(s.db)
	if err == nil {
		s.cameras = cams
	}
}

func (s *Server) handleListSettingsCameras(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cams, err := db.ListCameras(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	list := make([]cameraConfigDTO, len(cams))
	for i, c := range cams {
		list[i] = cameraToDTO(c)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleCreateCamera(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var req struct {
		ID                string           `json:"id"`
		RTSPURL           string           `json:"rtsp_url"`
		ChunkDuration     string           `json:"chunk_duration"`
		ReconnectInterval string           `json:"reconnect_interval"`
		VideoCodec        string           `json:"video_codec"`
		HasAudio          *bool            `json:"has_audio"`
		Width             int              `json:"width"`
		Height            int              `json:"height"`
		DisplayOrder      int              `json:"display_order"`
		HLSVideoMode      string           `json:"hls_video_mode"`
		RecordVideoMode   string           `json:"record_video_mode"`
		HLSSegmentSeconds *int             `json:"hls_segment_seconds"`
		HLSListSize       *int             `json:"hls_list_size"`
		Motion            *motionConfigDTO `json:"motion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.RTSPURL == "" {
		http.Error(w, "id and rtsp_url are required", http.StatusBadRequest)
		return
	}
	if req.Motion != nil {
		if err := validateMotionConfig(req.Motion); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	cam := config.CameraConfig{
		ID:                req.ID,
		RTSPURL:           req.RTSPURL,
		VideoCodec:        req.VideoCodec,
		HasAudio:          req.HasAudio,
		Width:             req.Width,
		Height:            req.Height,
		DisplayOrder:      req.DisplayOrder,
		HLSVideoMode:      req.HLSVideoMode,
		RecordVideoMode:   req.RecordVideoMode,
		HLSSegmentSeconds: req.HLSSegmentSeconds,
		HLSListSize:       req.HLSListSize,
	}
	if req.ChunkDuration != "" {
		d, err := time.ParseDuration(req.ChunkDuration)
		if err != nil {
			http.Error(w, "invalid chunk_duration: "+err.Error(), http.StatusBadRequest)
			return
		}
		cam.ChunkDuration = config.Duration(d)
	}
	if req.ReconnectInterval != "" {
		d, err := time.ParseDuration(req.ReconnectInterval)
		if err != nil {
			http.Error(w, "invalid reconnect_interval: "+err.Error(), http.StatusBadRequest)
			return
		}
		cam.ReconnectInterval = config.Duration(d)
	}

	// When all stream fields are "auto", probe the stream before inserting so
	// the DB stores real values from the start instead of showing "auto" in the UI.
	if s.prober != nil && req.VideoCodec == "" && req.HasAudio == nil && req.Width == 0 && req.Height == 0 {
		info := ffprobe.Resolve(r.Context(), ffprobe.Resolver{RTSPURL: req.RTSPURL}, s.prober, s.log)
		cam.VideoCodec = info.VideoCodec
		cam.HasAudio = &info.HasAudio
		cam.Width = info.Width
		cam.Height = info.Height
	}

	var motion *config.MotionConfig
	if req.Motion != nil {
		motion = &config.MotionConfig{
			Enabled:             req.Motion.Enabled,
			Threshold:           req.Motion.Threshold,
			FPS:                 req.Motion.FPS,
			CooldownSeconds:     req.Motion.CooldownSeconds,
			CaptureWidth:        req.Motion.CaptureWidth,
			CaptureHeight:       req.Motion.CaptureHeight,
			PlaybackLeadSeconds: req.Motion.PlaybackLeadSeconds,
		}
	}

	if err := db.CreateCamera(s.db, cam, motion); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "camera id already exists", http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.reloadCamerasFromDB()

	created, err := db.GetCamera(s.db, req.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if s.onCameraStart != nil {
		go s.onCameraStart(created)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cameraToDTO(created))
}

func (s *Server) handleUpdateCamera(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id := r.PathValue("id")

	existing, err := db.GetCamera(s.db, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var req struct {
		RTSPURL           string           `json:"rtsp_url"`
		ChunkDuration     string           `json:"chunk_duration"`
		ReconnectInterval string           `json:"reconnect_interval"`
		VideoCodec        string           `json:"video_codec"`
		HasAudio          *bool            `json:"has_audio"`
		Width             int              `json:"width"`
		Height            int              `json:"height"`
		DisplayOrder      int              `json:"display_order"`
		HLSVideoMode      string           `json:"hls_video_mode"`
		RecordVideoMode   string           `json:"record_video_mode"`
		HLSSegmentSeconds *int             `json:"hls_segment_seconds"`
		HLSListSize       *int             `json:"hls_list_size"`
		Motion            *motionConfigDTO `json:"motion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.RTSPURL == "" {
		http.Error(w, "rtsp_url is required", http.StatusBadRequest)
		return
	}
	// If the client submitted the masked URL (password == "xxxxx" from Redacted()),
	// restore the real password from the existing record but keep everything else
	// (host, path, query) from the submitted URL — so host changes are preserved.
	if u, err := url.Parse(req.RTSPURL); err == nil && u.User != nil {
		if pass, hasPass := u.User.Password(); hasPass && pass == "xxxxx" {
			if orig, err2 := url.Parse(existing.RTSPURL); err2 == nil && orig.User != nil {
				origPass, _ := orig.User.Password()
				u.User = url.UserPassword(orig.User.Username(), origPass)
				req.RTSPURL = u.String()
			} else {
				req.RTSPURL = existing.RTSPURL
			}
		}
	}
	if req.Motion != nil {
		if err := validateMotionConfig(req.Motion); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	cam := config.CameraConfig{
		ID:                id,
		RTSPURL:           req.RTSPURL,
		VideoCodec:        req.VideoCodec,
		HasAudio:          req.HasAudio,
		Width:             req.Width,
		Height:            req.Height,
		DisplayOrder:      req.DisplayOrder,
		HLSVideoMode:      req.HLSVideoMode,
		RecordVideoMode:   req.RecordVideoMode,
		HLSSegmentSeconds: req.HLSSegmentSeconds,
		HLSListSize:       req.HLSListSize,
	}
	if req.ChunkDuration != "" {
		d, err := time.ParseDuration(req.ChunkDuration)
		if err != nil {
			http.Error(w, "invalid chunk_duration: "+err.Error(), http.StatusBadRequest)
			return
		}
		cam.ChunkDuration = config.Duration(d)
	}
	if req.ReconnectInterval != "" {
		d, err := time.ParseDuration(req.ReconnectInterval)
		if err != nil {
			http.Error(w, "invalid reconnect_interval: "+err.Error(), http.StatusBadRequest)
			return
		}
		cam.ReconnectInterval = config.Duration(d)
	}

	var motion *config.MotionConfig
	if req.Motion != nil {
		motion = &config.MotionConfig{
			Enabled:             req.Motion.Enabled,
			Threshold:           req.Motion.Threshold,
			FPS:                 req.Motion.FPS,
			CooldownSeconds:     req.Motion.CooldownSeconds,
			CaptureWidth:        req.Motion.CaptureWidth,
			CaptureHeight:       req.Motion.CaptureHeight,
			PlaybackLeadSeconds: req.Motion.PlaybackLeadSeconds,
		}
	}

	if err := db.UpdateCamera(s.db, cam, motion); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.reloadCamerasFromDB()

	updated, err := db.GetCamera(s.db, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if s.onCameraStop != nil || s.onCameraStart != nil {
		snap := updated
		go func() {
			if s.onCameraStop != nil {
				s.onCameraStop(id)
			}
			if s.onCameraStart != nil {
				s.onCameraStart(snap)
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cameraToDTO(updated))
}

func (s *Server) handleDeleteCamera(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id := r.PathValue("id")
	deleteData := r.URL.Query().Get("delete_data") == "true"

	if _, err := db.GetCamera(s.db, id); err != nil {
		http.NotFound(w, r)
		return
	}

	if err := db.DeleteCamera(s.db, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if s.onCameraStop != nil {
		s.onCameraStop(id)
	}

	if deleteData {
		if s.storageCfg.Path != "" {
			_ = os.RemoveAll(filepath.Join(s.storageCfg.Path, id))
		}
		if s.cfg.SegmentsPath != "" {
			_ = os.RemoveAll(filepath.Join(s.cfg.SegmentsPath, id))
		}
	}

	s.reloadCamerasFromDB()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCameraStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	basePath := s.storageCfg.Path
	if basePath == "" {
		basePath = s.cfg.RecordingsPath
	}

	var totalBytes int64
	var totalChunks int
	var totalSeconds float64

	camDir := filepath.Join(basePath, id)
	_ = filepath.WalkDir(camDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".mp4") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		totalBytes += info.Size()
		totalChunks++

		// chunk duration from filename timestamps is unreliable; use chunk size as proxy.
		// Instead, we count file count and let the UI compute estimated hours from chunk_duration.
		_ = info
		return nil
	})

	// Estimate recording hours: each chunk is nominally chunk_duration seconds.
	// Look up chunk_duration from the camera config.
	chunkSec := 300.0 // default 5m
	if s.db != nil {
		if cam, err := db.GetCamera(s.db, id); err == nil {
			chunkSec = cam.EffectiveChunkDuration().Seconds()
		}
	}
	totalSeconds = float64(totalChunks) * chunkSec

	// Motion event count from DB.
	var motionCount int64
	if s.db != nil {
		motionCount, _ = db.CountMotionEvents(s.db, id)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"total_bytes":          totalBytes,
		"total_chunks":         totalChunks,
		"total_seconds":        totalSeconds,
		"total_motion_events":  motionCount,
	})
}
