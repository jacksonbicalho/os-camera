package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"camera/internal/db"
)

func (s *Server) handleCreateAnnotation(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	var body struct {
		Label string  `json:"label"`
		BboxX float64 `json:"bbox_x"`
		BboxY float64 `json:"bbox_y"`
		BboxW float64 `json:"bbox_w"`
		BboxH float64 `json:"bbox_h"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	id, err := db.InsertAnnotation(s.db, db.Annotation{
		EventID: eventID,
		Label:   body.Label,
		BboxX:   body.BboxX,
		BboxY:   body.BboxY,
		BboxW:   body.BboxW,
		BboxH:   body.BboxH,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (s *Server) handleListAnnotations(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	eventID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid event id", http.StatusBadRequest)
		return
	}
	list, err := db.ListAnnotationsByEvent(s.db, eventID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []db.Annotation{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) handleAnnotationCount(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	bboxCount, err := db.CountAnnotations(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	labelCount, err := db.CountLabeledEvents(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"count":        bboxCount,
		"label_count":  labelCount,
	})
}

func (s *Server) handleDeleteAnnotation(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := db.DeleteAnnotation(s.db, id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
