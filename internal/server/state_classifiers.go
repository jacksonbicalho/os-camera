package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"camera/internal/db"
	"camera/internal/stateclass"
)

func (s *Server) handleStateClassifiersGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	out := []stateclass.Classifier{}
	if s.db != nil {
		var err error
		out, err = db.ListStateClassifiers(s.db, id)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// decodeClassifier reads the body, pins camera/id from the URL, and validates.
func decodeClassifier(w http.ResponseWriter, r *http.Request, cameraID string, id int64) (stateclass.Classifier, bool) {
	var c stateclass.Classifier
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return c, false
	}
	c.CameraID = cameraID
	c.ID = id
	if c.Model == "" {
		c.Model = "custom-cls"
	}
	if err := c.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return c, false
	}
	return c, true
}

func (s *Server) handleStateClassifierCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		http.Error(w, "banco de dados não configurado", http.StatusServiceUnavailable)
		return
	}
	c, ok := decodeClassifier(w, r, id, 0)
	if !ok {
		return
	}
	newID, err := db.CreateStateClassifier(s.db, c)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	c.ID = newID
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(c)
}

func (s *Server) handleStateClassifierUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	if s.db == nil {
		http.Error(w, "banco de dados não configurado", http.StatusServiceUnavailable)
		return
	}
	c, ok := decodeClassifier(w, r, id, cid)
	if !ok {
		return
	}
	if err := db.UpdateStateClassifier(s.db, c); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (s *Server) handleStateClassifierDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	if s.db != nil {
		if err := db.DeleteStateClassifier(s.db, cid); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStateClassifierState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cid, err := strconv.ParseInt(r.PathValue("cid"), 10, 64)
	if !s.cameraExists(id) || err != nil {
		http.NotFound(w, r)
		return
	}
	var st *stateclass.State
	if s.db != nil {
		st, err = db.GetCurrentState(s.db, cid)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(st) // nil → null (estado ainda não escrito pela S3)
}
