package dbbackup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func makeDB(t *testing.T, path string) {
	t.Helper()
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	if _, err := d.Exec(`CREATE TABLE t(x INTEGER); INSERT INTO t VALUES (42);`); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestSnapshot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "camera.db")
	makeDB(t, src)
	dest := filepath.Join(dir, "backups")

	snap, err := Snapshot(src, dest, "v1.2.3-dev")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if filepath.Dir(snap) != dest {
		t.Errorf("snapshot fora do destDir: %s", snap)
	}
	base := filepath.Base(snap)
	if len(base) < len("backup-") || base[:len("backup-")] != "backup-" {
		t.Errorf("nome inesperado: %s", base)
	}

	// O snapshot é um DB válido e contém o dado.
	d, err := sql.Open("sqlite", snap)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer d.Close()
	var x int
	if err := d.QueryRow("SELECT x FROM t").Scan(&x); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}
	if x != 42 {
		t.Errorf("x = %d, quero 42", x)
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	// Nomes em ordem lexicográfica crescente = mais antigo → mais novo.
	names := []string{
		"backup-20260101000000-v1.db",
		"backup-20260102000000-v1.db",
		"backup-20260103000000-v1.db",
		"backup-20260104000000-v1.db",
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Arquivo não-backup não deve ser tocado.
	os.WriteFile(filepath.Join(dir, "outro.txt"), []byte("y"), 0o644)

	if err := Prune(dir, 2); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	for _, n := range []string{"backup-20260101000000-v1.db", "backup-20260102000000-v1.db"} {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("%s deveria ter sido removido", n)
		}
	}
	for _, n := range []string{"backup-20260103000000-v1.db", "backup-20260104000000-v1.db", "outro.txt"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s deveria existir: %v", n, err)
		}
	}
}

func TestRestore(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, "snap.db")
	makeDB(t, snap)

	target := filepath.Join(dir, "camera.db")
	if err := os.WriteFile(target, []byte("conteudo velho"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Sidecars de WAL que precisam sumir no restore.
	os.WriteFile(target+"-wal", []byte("wal velho"), 0o644)
	os.WriteFile(target+"-shm", []byte("shm velho"), 0o644)

	if err := Restore(snap, target); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	for _, side := range []string{target + "-wal", target + "-shm"} {
		if _, err := os.Stat(side); !os.IsNotExist(err) {
			t.Errorf("sidecar %s deveria ter sido removido", side)
		}
	}

	d, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatalf("open restaurado: %v", err)
	}
	defer d.Close()
	var x int
	if err := d.QueryRow("SELECT x FROM t").Scan(&x); err != nil {
		t.Fatalf("query restaurado: %v", err)
	}
	if x != 42 {
		t.Errorf("x = %d, quero 42 (conteúdo do snapshot)", x)
	}
}
