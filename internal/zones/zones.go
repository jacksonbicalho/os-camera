package zones

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sync"
)

type Zone struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Store struct {
	mu   sync.RWMutex
	path string
	data map[string][]Zone
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path, data: make(map[string][]Zone)}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, nil
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		slog.Warn("motion_zones.json corrompido, ignorando", "path", path, "error", err)
		s.data = make(map[string][]Zone)
	}
	return s, nil
}

func (s *Store) Get(cameraID string) []Zone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	z := s.data[cameraID]
	if z == nil {
		return []Zone{}
	}
	return z
}

func (s *Store) Set(cameraID string, z []Zone) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if z == nil {
		z = []Zone{}
	}
	s.data[cameraID] = z
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}
