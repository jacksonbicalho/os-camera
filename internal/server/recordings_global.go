package server

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"camera/internal/db"
)

// handleGlobalRecordings lista as gravações (chunks da tabela recordings) de TODAS as
// câmeras acessíveis ao usuário num dia, para o modo "Gravações" da /recordings.
// Diferente de /api/moments (que só traz eventos), aqui aparece a cobertura de gravação
// inteira. Filtros: `cameras` (csv), `window` (1|2|4|6|12|24h, default 24) e
// `motion_only`. Ordena por start desc. Cada item aponta para a câmera + instante.
func (s *Server) handleGlobalRecordings(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}
	localDay, err := time.ParseInLocation("2006-01-02", r.URL.Query().Get("date"), loc)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	dayStart := localDay.UTC()
	dayEnd := localDay.Add(24 * time.Hour).UTC()

	// Janela: âncora no fim do dia (ou "agora", se o dia for hoje) recuando window horas.
	window := 24
	if v, err := strconv.Atoi(r.URL.Query().Get("window")); err == nil && v > 0 {
		window = v
	}
	anchor := dayEnd
	if now := time.Now().UTC(); now.Before(anchor) {
		anchor = now
	}
	start := anchor.Add(-time.Duration(window) * time.Hour)
	if start.Before(dayStart) {
		start = dayStart
	}

	motionOnly := r.URL.Query().Get("motion_only") == "true"

	cams, err := db.ListCameras(s.db)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	ac, _ := r.Context().Value(claimsKey).(authClaims)
	allowed := map[string]struct{}{}
	if ac.Role != "admin" {
		if ids, err := db.GetUserCameras(s.db, ac.UserID); err == nil {
			for _, id := range ids {
				allowed[id] = struct{}{}
			}
		}
	}
	var camFilter map[string]struct{}
	if cv := r.URL.Query().Get("cameras"); cv != "" {
		camFilter = map[string]struct{}{}
		for _, id := range strings.Split(cv, ",") {
			camFilter[strings.TrimSpace(id)] = struct{}{}
		}
	}

	names := map[string]string{}
	ids := make([]string, 0, len(cams))
	for _, c := range cams {
		if ac.Role != "admin" {
			if _, ok := allowed[c.ID]; !ok {
				continue
			}
		}
		if camFilter != nil {
			if _, ok := camFilter[c.ID]; !ok {
				continue
			}
		}
		names[c.ID] = c.Name
		ids = append(ids, c.ID)
	}

	recs, err := db.ListRecordingsInRange(s.db, ids, start, anchor, motionOnly)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type recording struct {
		ID         int64  `json:"id"`
		CameraID   string `json:"camera_id"`
		CameraName string `json:"camera_name"`
		Start      string `json:"start"`
		HasMotion  bool   `json:"has_motion"`
		URL        string `json:"url"`
	}
	out := make([]recording, 0, len(recs))
	for _, rec := range recs {
		url := ""
		if rel, err := filepath.Rel(s.cfg.RecordingsPath, rec.Path); err == nil {
			url = "/recordings/" + filepath.ToSlash(rel)
		}
		out = append(out, recording{
			ID: rec.ID, CameraID: rec.CameraID, CameraName: names[rec.CameraID],
			Start: rec.StartedAt.UTC().Format(time.RFC3339), HasMotion: rec.HasMotion, URL: url,
		})
	}
	writeJSON(w, map[string]any{"recordings": out, "total": len(out)})
}
