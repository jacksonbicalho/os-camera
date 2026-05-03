package motion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type store struct {
	basePath string
}

func newStore(basePath string) *store {
	return &store{basePath: basePath}
}

func (s *store) record(cameraID string, ts time.Time, score float64) error {
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

	return json.NewEncoder(f).Encode(map[string]any{
		"time":  ts.UTC().Format(time.RFC3339),
		"score": score,
	})
}
