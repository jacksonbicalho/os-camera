package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"camera/internal/db"
)

// contentKind normaliza o querystring kind para um dos valores aceitos por
// db.ContentDays (default "all").
func contentKind(r *http.Request) string {
	switch r.URL.Query().Get("kind") {
	case db.ContentRecordings:
		return db.ContentRecordings
	case db.ContentEvents:
		return db.ContentEvents
	default:
		return db.ContentAll
	}
}

// handleContentDays devolve as datas locais (YYYY-MM-DD) em que a câmera tem
// conteúdo do tipo `kind` (recordings/events/all) — os calendários usam para
// habilitar só os dias com conteúdo.
func (s *Server) handleContentDays(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	days := []string{}
	if s.db != nil {
		if d, err := db.ContentDays(s.db, id, s.contentLocation(), contentKind(r)); err != nil {
			s.log.Warn("content days", "camera", id, "error", err)
		} else {
			days = d
		}
	}

	writeContentDays(w, days)
}

// handleGlobalContentDays agrega os dias com conteúdo de várias câmeras (as
// acessíveis ao usuário, intersectadas com o filtro opcional cameras=).
func (s *Server) handleGlobalContentDays(w http.ResponseWriter, r *http.Request) {
	days := []string{}
	if s.db != nil {
		ids := s.accessibleCameraIDs(r)
		if cv := r.URL.Query().Get("cameras"); cv != "" {
			filter := map[string]struct{}{}
			for _, id := range strings.Split(cv, ",") {
				filter[strings.TrimSpace(id)] = struct{}{}
			}
			kept := ids[:0]
			for _, id := range ids {
				if _, ok := filter[id]; ok {
					kept = append(kept, id)
				}
			}
			ids = kept
		}
		if d, err := db.ContentDaysMulti(s.db, ids, s.contentLocation(), contentKind(r)); err != nil {
			s.log.Warn("content days multi", "error", err)
		} else {
			days = d
		}
	}

	writeContentDays(w, days)
}

// accessibleCameraIDs devolve os ids de câmera que o usuário pode ver: todas
// (admin) ou as concedidas (viewer). Mesmo padrão de handleGlobalRecordings.
func (s *Server) accessibleCameraIDs(r *http.Request) []string {
	cams, err := db.ListCameras(s.db)
	if err != nil {
		return nil
	}
	ac, _ := r.Context().Value(claimsKey).(authClaims)
	var allowed map[string]struct{}
	if ac.Role != "admin" {
		allowed = map[string]struct{}{}
		if granted, err := db.GetUserCameras(s.db, ac.UserID); err == nil {
			for _, id := range granted {
				allowed[id] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(cams))
	for _, c := range cams {
		if allowed != nil {
			if _, ok := allowed[c.ID]; !ok {
				continue
			}
		}
		ids = append(ids, c.ID)
	}
	return ids
}

func (s *Server) contentLocation() *time.Location {
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func writeContentDays(w http.ResponseWriter, days []string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"days": days})
}
