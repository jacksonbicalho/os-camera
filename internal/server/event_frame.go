package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// findChunkForTime acha o chunk de gravação que cobre o instante t (o último que
// começa em t ou antes, no dia UTC de t) e o offset em segundos dentro dele.
// Usado para extrair um frame LIMPO do MP4 (a gravação não tem as anotações de
// movimento que ficam nos _motion.jpg).
func findChunkForTime(recordingsPath, cameraID string, t time.Time) (path string, offsetSeconds float64, ok bool) {
	dir := filepath.Join(recordingsPath, cameraID, t.UTC().Format("2006/01/02"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", 0, false
	}
	var bestStart time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mp4") {
			continue
		}
		ts, err := time.ParseInLocation("20060102150405", strings.TrimSuffix(e.Name(), ".mp4"), time.UTC)
		if err != nil {
			continue
		}
		if !ts.After(t) && ts.After(bestStart) {
			bestStart = ts
			path = filepath.Join(dir, e.Name())
		}
	}
	if path == "" {
		return "", 0, false
	}
	return path, t.Sub(bestStart).Seconds(), true
}

// handleEventFrame devolve um frame LIMPO (sem o bbox de movimento) extraído da
// gravação no instante `time` (RFC3339). Usado pelo picker de estados ao escolher.
func (s *Server) handleEventFrame(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.cameraExists(id) {
		http.NotFound(w, r)
		return
	}
	if s.frameFn == nil {
		http.Error(w, "frame extraction not available", http.StatusServiceUnavailable)
		return
	}
	t, err := time.Parse(time.RFC3339, r.URL.Query().Get("time"))
	if err != nil {
		http.Error(w, "invalid time", http.StatusBadRequest)
		return
	}
	path, offset, ok := findChunkForTime(s.cfg.RecordingsPath, id, t)
	if !ok {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	data, err := s.frameFn(ctx, path, offset)
	if err != nil || len(data) == 0 {
		http.Error(w, "frame extraction failed", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(data)
}
