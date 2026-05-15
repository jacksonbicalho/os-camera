package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/ffprobe"
)

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
	Enabled         bool    `json:"enabled"`
	Threshold       float64 `json:"threshold"`
	FPS             int     `json:"fps"`
	CooldownSeconds int     `json:"cooldown_seconds"`
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
	}
	if cam.Motion != nil {
		dto.Motion = &motionConfigDTO{
			Enabled:         cam.Motion.Enabled,
			Threshold:       cam.Motion.Threshold,
			FPS:             cam.Motion.FPS,
			CooldownSeconds: cam.Motion.CooldownSeconds,
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

	cam := config.CameraConfig{
		ID:           req.ID,
		RTSPURL:      req.RTSPURL,
		VideoCodec:   req.VideoCodec,
		HasAudio:     req.HasAudio,
		Width:        req.Width,
		Height:       req.Height,
		DisplayOrder: req.DisplayOrder,
	}
	if req.ChunkDuration != "" {
		if d, err := time.ParseDuration(req.ChunkDuration); err == nil {
			cam.ChunkDuration = config.Duration(d)
		}
	}
	if req.ReconnectInterval != "" {
		if d, err := time.ParseDuration(req.ReconnectInterval); err == nil {
			cam.ReconnectInterval = config.Duration(d)
		}
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
			Enabled:         req.Motion.Enabled,
			Threshold:       req.Motion.Threshold,
			FPS:             req.Motion.FPS,
			CooldownSeconds: req.Motion.CooldownSeconds,
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

	if _, err := db.GetCamera(s.db, id); err != nil {
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

	cam := config.CameraConfig{
		ID:           id,
		RTSPURL:      req.RTSPURL,
		VideoCodec:   req.VideoCodec,
		HasAudio:     req.HasAudio,
		Width:        req.Width,
		Height:       req.Height,
		DisplayOrder: req.DisplayOrder,
	}
	if req.ChunkDuration != "" {
		if d, err := time.ParseDuration(req.ChunkDuration); err == nil {
			cam.ChunkDuration = config.Duration(d)
		}
	}
	if req.ReconnectInterval != "" {
		if d, err := time.ParseDuration(req.ReconnectInterval); err == nil {
			cam.ReconnectInterval = config.Duration(d)
		}
	}

	var motion *config.MotionConfig
	if req.Motion != nil {
		motion = &config.MotionConfig{
			Enabled:         req.Motion.Enabled,
			Threshold:       req.Motion.Threshold,
			FPS:             req.Motion.FPS,
			CooldownSeconds: req.Motion.CooldownSeconds,
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

	if deleteData && s.storageCfg.Path != "" {
		_ = os.RemoveAll(filepath.Join(s.storageCfg.Path, id))
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
