package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"camera/internal/db"
)

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
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	unlabeled := r.URL.Query().Get("unlabeled") == "true"
	offset := (page - 1) * limit

	events, total, err := db.PageMotionEvents(s.db, cameraID, offset, limit, unlabeled)
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
