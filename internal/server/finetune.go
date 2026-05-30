package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"camera/internal/analysis"
	"camera/internal/db"
)

func (s *Server) handleStartFinetune(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cfg, err := db.GetVideoAnalysisConfig(s.db)
	if err != nil || cfg.ServiceURL == "" {
		http.Error(w, "analysis service not configured", http.StatusServiceUnavailable)
		return
	}

	epochs := 20
	var body struct {
		Epochs int `json:"epochs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Epochs > 0 && body.Epochs <= 200 {
		epochs = body.Epochs
	}

	annotations, err := db.ListAllAnnotations(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	labeledEvents, err := db.ListLabeledEvents(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(annotations) == 0 && len(labeledEvents) == 0 {
		http.Error(w, "no annotations available", http.StatusBadRequest)
		return
	}

	items := make([]analysis.AnnotationItem, 0, len(annotations)+len(labeledEvents))
	for _, a := range annotations {
		ev, err := db.GetMotionEventByID(s.db, a.EventID)
		if err != nil || ev.FramePath == "" {
			continue
		}
		datePart := ev.OccurredAt.UTC().Format("2006/01/02")
		imagePath := filepath.Join(s.cfg.RecordingsPath, ev.CameraID, datePart, ev.FramePath)
		items = append(items, analysis.AnnotationItem{
			ImagePath: imagePath,
			Label:     a.Label,
			BboxX:     a.BboxX,
			BboxY:     a.BboxY,
			BboxW:     a.BboxW,
			BboxH:     a.BboxH,
		})
	}
	// Text-labeled events without manual bounding boxes use a full-image bbox.
	for _, ev := range labeledEvents {
		datePart := ev.OccurredAt.UTC().Format("2006/01/02")
		imagePath := filepath.Join(s.cfg.RecordingsPath, ev.CameraID, datePart, ev.FramePath)
		items = append(items, analysis.AnnotationItem{
			ImagePath: imagePath,
			Label:     ev.Label,
			BboxX:     0.5,
			BboxY:     0.5,
			BboxW:     1.0,
			BboxH:     1.0,
		})
	}
	if len(items) == 0 {
		http.Error(w, "no annotations with associated snapshots", http.StatusBadRequest)
		return
	}

	baseModel := "yolov8n"
	for _, part := range strings.Split(cfg.Model, "+") {
		if p := strings.TrimSpace(part); p != "custom" && p != "" {
			baseModel = p
			break
		}
	}
	client := analysis.NewClient(cfg.ServiceURL)
	resp, err := client.Finetune(context.Background(), analysis.FinetuneRequest{
		Annotations: items,
		BaseModel:   baseModel,
		Epochs:      epochs,
	})
	if err != nil {
		http.Error(w, "finetune request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleCancelFinetune(w http.ResponseWriter, r *http.Request) {
	cfg, err := db.GetVideoAnalysisConfig(s.db)
	if err != nil || cfg.ServiceURL == "" {
		http.Error(w, "analysis service not configured", http.StatusServiceUnavailable)
		return
	}
	jobID := r.PathValue("job_id")
	client := analysis.NewClient(cfg.ServiceURL)
	_ = client.CancelFinetune(r.Context(), jobID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFinetuneStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cfg, err := db.GetVideoAnalysisConfig(s.db)
	if err != nil || cfg.ServiceURL == "" {
		http.Error(w, "analysis service not configured", http.StatusServiceUnavailable)
		return
	}

	jobID := r.PathValue("job_id")
	client := analysis.NewClient(cfg.ServiceURL)
	status, err := client.FinetuneStatus(context.Background(), jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if status.Status == "done" || status.Status == "completed" {
		_ = db.SetHasCustomModel(s.db, true)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
