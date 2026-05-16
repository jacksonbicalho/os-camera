package motion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type store struct {
	basePath string
	onEvent  func(cameraID string, t time.Time, score float64, frame, label, color string, bbox BBox)
}

func newStore(basePath string, onEvent func(cameraID string, t time.Time, score float64, frame, label, color string, bbox BBox)) *store {
	return &store{basePath: basePath, onEvent: onEvent}
}

func (s *store) record(cameraID string, ts time.Time, score float64, frame, label, color string, bbox BBox) error {
	dir := filepath.Join(s.basePath, cameraID, ts.UTC().Format("2006/01/02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "motion.ndjson")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := map[string]any{
		"time":  ts.UTC().Format(time.RFC3339),
		"score": score,
		"bbox":  map[string]float64{"x": bbox.X, "y": bbox.Y, "w": bbox.W, "h": bbox.H},
	}
	if frame != "" {
		entry["frame"] = frame
	}
	if label != "" {
		entry["label"] = label
	}
	if color != "" {
		entry["color"] = color
	}
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		return err
	}
	if s.onEvent != nil {
		s.onEvent(cameraID, ts, score, frame, label, color, bbox)
	}
	return nil
}

func (s *store) saveJPEG(cameraID string, ts time.Time, data []byte) (string, error) {
	dir := filepath.Join(s.basePath, cameraID, ts.UTC().Format("2006/01/02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := ts.UTC().Format("20060102150405") + "_motion.jpg"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return name, nil
}
