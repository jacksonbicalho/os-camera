package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"camera/internal/db"
)

// handleMoments lista "momentos" (eventos de movimento + transições de estado) de TODAS
// as câmeras acessíveis ao usuário num dia, para o navegador global de gravações
// (/recordings). Ordena por tempo desc e pagina. Filtros opcionais: `cameras` (csv de
// ids) e `category` (movimento|pessoa|ia|estados). Cada momento aponta para a câmera +
// instante, e o frontend abre a CameraPage ali (deep-link de seek).
func (s *Server) handleMoments(w http.ResponseWriter, r *http.Request) {
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
	catFilter := r.URL.Query().Get("category")
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	type moment struct {
		CameraID   string  `json:"camera_id"`
		CameraName string  `json:"camera_name"`
		Time       string  `json:"time"`
		Kind       string  `json:"kind"`
		Label      string  `json:"label,omitempty"`
		Category   string  `json:"category"`
		Frame      string  `json:"frame,omitempty"`
		Score      float64 `json:"score"`
	}
	// matchesQuery casa o termo de busca (já normalizado) por substring contra a label
	// ou o nome da categoria do momento. query vazia casa tudo.
	matchesQuery := func(label, category string) bool {
		if query == "" {
			return true
		}
		return strings.Contains(strings.ToLower(label), query) ||
			strings.Contains(strings.ToLower(category), query)
	}

	moments := []moment{}
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
		if catFilter == "" || catFilter != "estados" {
			if evs, err := db.ListMotionEvents(s.db, c.ID, dayStart, dayEnd); err == nil {
				for _, e := range evs {
					cat := db.MotionCategory(e.Label)
					if catFilter != "" && cat != catFilter {
						continue
					}
					if !matchesQuery(e.Label, cat) {
						continue
					}
					moments = append(moments, moment{
						CameraID: c.ID, CameraName: c.Name, Time: e.OccurredAt.UTC().Format(time.RFC3339),
						Kind: "motion", Label: e.Label, Category: cat, Frame: e.FramePath, Score: e.Score,
					})
				}
			}
		}
		if catFilter == "" || catFilter == "estados" {
			if trs, err := db.ListCameraStateTransitions(s.db, c.ID, dayStart, dayEnd); err == nil {
				for _, t := range trs {
					if !matchesQuery(t.State, "estados") {
						continue
					}
					moments = append(moments, moment{
						CameraID: c.ID, CameraName: c.Name, Time: t.ChangedAt.UTC().Format(time.RFC3339),
						Kind: "state", Label: t.State, Category: "estados", Frame: t.FramePath, Score: t.Confidence,
					})
				}
			}
		}
	}
	sort.Slice(moments, func(i, j int) bool { return moments[i].Time > moments[j].Time })

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 100
	}
	total := len(moments)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	writeJSON(w, map[string]any{
		"moments": moments[start:end],
		"total":   total,
		"hasMore": end < total,
	})
}
