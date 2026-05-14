package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"camera/internal/config"
	"camera/internal/db"
)

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
		ChunkDuration:     cam.EffectiveChunkDuration().String(),
		ReconnectInterval: cam.EffectiveReconnectInterval().String(),
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
		s.onCameraStart(created)
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

	if s.onCameraStop != nil {
		s.onCameraStop(id)
	}

	s.reloadCamerasFromDB()

	updated, err := db.GetCamera(s.db, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if s.onCameraStart != nil {
		s.onCameraStart(updated)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cameraToDTO(updated))
}

func (s *Server) handleDeleteCamera(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id := r.PathValue("id")

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

	s.reloadCamerasFromDB()
	w.WriteHeader(http.StatusNoContent)
}
