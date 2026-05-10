package zones_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"camera/internal/zones"
)

func TestNewStoreEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "motion_zones.json")
	s, err := zones.NewStore(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := s.Get("cam1")
	if len(got) != 0 {
		t.Fatalf("expected empty zones, got %v", got)
	}
}

func TestNewStoreReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "motion_zones.json")
	data := map[string][]zones.Zone{
		"garage": {{X: 0.1, Y: 0.2, W: 0.3, H: 0.4}},
	}
	b, _ := json.Marshal(data)
	os.WriteFile(path, b, 0o644)

	s, err := zones.NewStore(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := s.Get("garage")
	if len(got) != 1 || got[0].X != 0.1 || got[0].Y != 0.2 {
		t.Fatalf("unexpected zones: %v", got)
	}
}

func TestSetPersistsToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "motion_zones.json")
	s, _ := zones.NewStore(path)

	z := []zones.Zone{{X: 0.05, Y: 0.10, W: 0.30, H: 0.20}}
	if err := s.Set("cam1", z); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Reload from disk
	s2, err := zones.NewStore(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	got := s2.Get("cam1")
	if len(got) != 1 || got[0].W != 0.30 {
		t.Fatalf("unexpected zones after reload: %v", got)
	}
}

func TestSetEmptySlicePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "motion_zones.json")
	s, _ := zones.NewStore(path)
	s.Set("cam1", []zones.Zone{{X: 0.1, Y: 0.1, W: 0.1, H: 0.1}})
	s.Set("cam1", []zones.Zone{})

	s2, _ := zones.NewStore(path)
	got := s2.Get("cam1")
	if len(got) != 0 {
		t.Fatalf("expected empty after clear, got %v", got)
	}
}

func TestGetUnknownCameraReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "motion_zones.json")
	s, _ := zones.NewStore(path)
	got := s.Get("nonexistent")
	if got == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestSetMultipleCameras(t *testing.T) {
	path := filepath.Join(t.TempDir(), "motion_zones.json")
	s, _ := zones.NewStore(path)

	s.Set("cam1", []zones.Zone{{X: 0.0, Y: 0.0, W: 0.5, H: 0.5}})
	s.Set("cam2", []zones.Zone{{X: 0.5, Y: 0.5, W: 0.5, H: 0.5}})

	if len(s.Get("cam1")) != 1 {
		t.Fatal("cam1 zones missing")
	}
	if len(s.Get("cam2")) != 1 {
		t.Fatal("cam2 zones missing")
	}
}

func TestCorruptedFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "motion_zones.json")
	os.WriteFile(path, []byte("not json {{{"), 0o644)

	s, err := zones.NewStore(path)
	if err != nil {
		t.Fatalf("expected no error on corrupted file, got: %v", err)
	}
	if len(s.Get("cam1")) != 0 {
		t.Fatal("expected empty zones from corrupted file")
	}
}
