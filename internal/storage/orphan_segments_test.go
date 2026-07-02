package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"camera/internal/storage"
)

func TestCleanOrphanedSegments(t *testing.T) {
	dir := t.TempDir()
	// cam1 is a live camera; cam2 was deleted; a stray file is ignored.
	for _, id := range []string{"cam1", "cam2"} {
		if err := os.MkdirAll(filepath.Join(dir, id), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, id, "000000.ts"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write ts: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write stray: %v", err)
	}

	n := storage.CleanOrphanedSegments(dir, map[string]bool{"cam1": true}, discardLogger())
	if n != 1 {
		t.Fatalf("removed = %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "cam1")); err != nil {
		t.Errorf("cam1 dir should be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cam2")); !os.IsNotExist(err) {
		t.Errorf("cam2 dir should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stray.txt")); err != nil {
		t.Errorf("stray file should be untouched: %v", err)
	}
}

func TestCleanOrphanedSegments_EmptyPath(t *testing.T) {
	if n := storage.CleanOrphanedSegments("", nil, discardLogger()); n != 0 {
		t.Fatalf("empty path removed = %d, want 0", n)
	}
}
