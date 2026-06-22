package server

import (
	"net/http"
	"time"

	"camera/internal/db"
)

func parseReportTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", v)
}

// handleEventReport aggregates motion events over a period (default: last 7 days).
func (s *Server) handleEventReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireDB(w) {
		return
	}
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -7)
	to := now
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := parseReportTime(v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := parseReportTime(v); err == nil {
			to = t
		}
	}
	camera := r.URL.Query().Get("camera")
	rep, err := db.AggregateMotionEvents(s.db, from, to, camera)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, rep)
}
