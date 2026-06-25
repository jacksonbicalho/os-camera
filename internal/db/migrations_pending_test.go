package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasPendingMigrationsDoesNotCreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "naoexiste.db")

	pending, err := HasPendingMigrations(path)
	if err != nil {
		t.Fatalf("HasPendingMigrations: %v", err)
	}
	if !pending {
		t.Error("banco inexistente deveria reportar migrations pendentes")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("HasPendingMigrations não deveria criar o arquivo do banco")
	}
}

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
