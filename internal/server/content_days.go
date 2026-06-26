package server

import (
	"encoding/json"
	"net/http"
	"time"

	"camera/internal/db"
)

// handleContentDays devolve as datas locais (YYYY-MM-DD) em que a câmera tem
// gravação ou evento de movimento — os calendários usam para habilitar só os
// dias com conteúdo.
func (s *Server) handleContentDays(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	days := []string{}
	if s.db != nil {
		loc, err := time.LoadLocation(s.timezone)
		if err != nil {
			loc = time.UTC
		}
		if d, err := db.ContentDays(s.db, id, loc); err != nil {
			s.log.Warn("content days", "camera", id, "error", err)
		} else {
			days = d
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"days": days})
}
