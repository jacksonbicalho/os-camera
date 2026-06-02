package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"camera/internal/db"
)

const bulkEventMaxIDs = 500

func (s *Server) handleUpdateEventFrame(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	ev, err := db.GetMotionEventByID(s.db, id)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	framePath := ev.FramePath
	if framePath == "" {
		framePath = ev.OccurredAt.UTC().Format("20060102150405") + "_motion.jpg"
	}
	datePart := ev.OccurredAt.UTC().Format("2006/01/02")
	dir := filepath.Join(s.cfg.RecordingsPath, ev.CameraID, datePart)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.log.Error("mkdir event frame dir", "dir", dir, "err", err)
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}
	fullPath := filepath.Join(dir, framePath)
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		s.log.Error("write event frame", "path", fullPath, "err", err)
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}
	if ev.FramePath == "" {
		if err := db.UpdateMotionEventFramePath(s.db, id, framePath); err != nil {
			s.log.Warn("update event frame_path", "id", id, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateEventLabel(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := db.UpdateMotionEventLabel(s.db, id, body.Label); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePageEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	cameraID := r.PathValue("id")

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	unlabeled := r.URL.Query().Get("unlabeled") == "true"
	labelSearch := r.URL.Query().Get("label")
	dismissedOnly := r.URL.Query().Get("dismissed") == "true"
	offset := (page - 1) * limit

	events, total, err := db.PageMotionEvents(s.db, cameraID, offset, limit, unlabeled, labelSearch, dismissedOnly)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type eventItem struct {
		ID         int64   `json:"id"`
		OccurredAt string  `json:"time"`
		Score      float64 `json:"score"`
		FramePath  string  `json:"frame,omitempty"`
		Label      string  `json:"label,omitempty"`
	}

	items := make([]eventItem, len(events))
	for i, ev := range events {
		items[i] = eventItem{
			ID:         ev.ID,
			OccurredAt: ev.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
			Score:      ev.Score,
			FramePath:  ev.FramePath,
			Label:      ev.Label,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"events": items,
		"total":  total,
	})
}

func (s *Server) handleBulkDismissEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body.IDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"dismissed": 0})
		return
	}
	if len(body.IDs) > bulkEventMaxIDs {
		http.Error(w, "too many ids", http.StatusBadRequest)
		return
	}
	n, err := db.BulkDismissMotionEvents(s.db, body.IDs)
	if err != nil {
		s.log.Error("bulk dismiss events", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"dismissed": n})
}

func (s *Server) handleBulkDeleteEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body.IDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deleted": 0})
		return
	}
	if len(body.IDs) > bulkEventMaxIDs {
		http.Error(w, "too many ids", http.StatusBadRequest)
		return
	}
	deleted, snaps, err := db.BulkDeleteMotionEvents(s.db, body.IDs)
	if err != nil {
		s.log.Error("bulk delete events", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if s.storageCfg.Path != "" {
		for _, ev := range snaps {
			if ev.FramePath == "" {
				continue
			}
			day := ev.OccurredAt.UTC().Format("2006/01/02")
			p := filepath.Join(s.storageCfg.Path, ev.CameraID, filepath.FromSlash(day), ev.FramePath)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				s.log.Warn("delete motion jpeg", "path", p, "err", err)
			}
		}
	}
	if err := db.ResetHasMotionOrphaned(s.db, ""); err != nil {
		s.log.Warn("reset has_motion after bulk delete", "err", err)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"deleted": deleted})
}

func (s *Server) handleBulkUpdateEventLabels(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	var body struct {
		IDs   []int64 `json:"ids"`
		Label string  `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if len(body.IDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"updated": 0})
		return
	}
	if len(body.IDs) > bulkEventMaxIDs {
		http.Error(w, "too many ids", http.StatusBadRequest)
		return
	}
	updated, err := db.BulkUpdateMotionEventLabels(s.db, body.IDs, body.Label)
	if err != nil {
		s.log.Error("bulk update labels", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"updated": updated})
}
