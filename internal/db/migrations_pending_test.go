package db

import (
	"path/filepath"
	"testing"
)

func TestHasPendingMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "camera.db")

	// Banco novo: todas as migrations pendentes.
	pending, err := HasPendingMigrations(path)
	if err != nil {
		t.Fatalf("HasPendingMigrations (novo): %v", err)
	}
	if !pending {
		t.Error("banco novo deveria ter migrations pendentes")
	}

	// Open aplica todas.
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	d.Close()

	pending, err = HasPendingMigrations(path)
	if err != nil {
		t.Fatalf("HasPendingMigrations (após Open): %v", err)
	}
	if pending {
		t.Error("após Open não deveria haver migrations pendentes")
	}
}
