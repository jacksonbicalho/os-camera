package motion

import (
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
	if s.onEvent != nil {
		s.onEvent(cameraID, ts, score, frame, label, color, bbox)
	}
	return nil
}

func (s *store) saveJPEG(cameraID string, ts time.Time, data []byte) (name, fullPath string, err error) {
	dir := filepath.Join(s.basePath, cameraID, ts.UTC().Format("2006/01/02"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", "", err
	}
	name = ts.UTC().Format("20060102150405") + "_motion.jpg"
	fullPath = filepath.Join(dir, name)
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", "", err
	}
	return name, fullPath, nil
}
