package server

import (
	"encoding/json"
	"net/http"

	"camera/internal/db"
)

func (s *Server) handleGetAnalysisConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cfg, err := db.GetVideoAnalysisConfig(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleUpdateAnalysisConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var cfg db.VideoAnalysisConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := db.UpdateVideoAnalysisConfig(s.db, cfg); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cfg)
}

func (s *Server) handleGetCameraAnalysisConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cameraID := r.PathValue("id")
	enabled, err := db.GetCameraAnalysisEnabled(s.db, cameraID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": enabled})
}

func (s *Server) handleUpdateCameraAnalysisConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cameraID := r.PathValue("id")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := db.SetCameraAnalysisEnabled(s.db, cameraID, body.Enabled); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": body.Enabled})
}
