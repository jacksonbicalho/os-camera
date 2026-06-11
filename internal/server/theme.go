package server

import (
	"encoding/json"
	"net/http"

	"camera/internal/db"
)

// validThemes are the UI theme preferences the frontend accepts. "system" follows
// the OS prefers-color-scheme; the frontend resolves it to dark/light at render.
var validThemes = map[string]bool{"dark": true, "light": true, "system": true}

func (s *Server) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	theme, err := db.GetUserTheme(s.db, s.currentUserID(r))
	if err != nil {
		http.Error(w, "failed to load preferences", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"theme": theme})
}

func (s *Server) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "database unavailable", http.StatusInternalServerError)
		return
	}
	var body struct {
		Theme string `json:"theme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if !validThemes[body.Theme] {
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}
	if err := db.SetUserTheme(s.db, s.currentUserID(r), body.Theme); err != nil {
		http.Error(w, "failed to save preferences", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
