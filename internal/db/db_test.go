package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"camera/internal/db"
)

func TestOpen_CreatesSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	tables := []string{"users", "user_cameras", "cameras", "camera_motion", "system_config", "schema_migrations"}
	for _, tbl := range tables {
		var name string
		err := database.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("tabela %q não encontrada: %v", tbl, err)
		}
	}
}

func TestOpen_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db1, err := db.Open(path)
	if err != nil {
		t.Fatalf("primeira abertura: %v", err)
	}
	db1.Close()

	db2, err := db.Open(path)
	if err != nil {
		t.Fatalf("segunda abertura: %v", err)
	}
	db2.Close()
}

func TestOpen_IsNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "novo.db")

	// arquivo não existe → IsNew deve ser verdadeiro
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		t.Fatal("esperava arquivo inexistente")
	}

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if !database.IsNew {
		t.Error("IsNew deveria ser true para banco recém-criado")
	}

	// abrir novamente → IsNew false
	database2, err := db.Open(path)
	if err != nil {
		t.Fatalf("segunda abertura: %v", err)
	}
	defer database2.Close()

	if database2.IsNew {
		t.Error("IsNew deveria ser false para banco já existente")
	}
}
