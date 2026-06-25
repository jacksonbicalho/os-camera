package updater

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEvaluateBoot(t *testing.T) {
	trial, m := EvaluateBoot(Marker{State: "pending", Attempts: 0})
	if trial != ActionTrial {
		t.Errorf("attempts 0 → %v, quero Trial", trial)
	}
	if m.Attempts != 1 {
		t.Errorf("attempts após trial = %d, quero 1", m.Attempts)
	}

	rb, _ := EvaluateBoot(Marker{State: "pending", Attempts: 1})
	if rb != ActionRollback {
		t.Errorf("attempts 1 → %v, quero Rollback", rb)
	}
}

func TestRollback(t *testing.T) {
	dir := t.TempDir()

	// Snapshot SQLite real com um dado conhecido.
	snap := filepath.Join(dir, "snap.db")
	d, err := sql.Open("sqlite", snap)
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(`CREATE TABLE t(x INTEGER); INSERT INTO t VALUES (7);`)
	d.Close()

	// Binário "antigo" guardado e o atual (novo/quebrado).
	backup := filepath.Join(dir, "camera.old")
	target := filepath.Join(dir, "camera")
	os.WriteFile(backup, []byte("BINARIO ANTIGO"), 0o755)
	os.WriteFile(target, []byte("BINARIO NOVO QUEBRADO"), 0o755)

	// DB atual "corrompido pela migration" que precisa voltar.
	dbPath := filepath.Join(dir, "camera.db")
	os.WriteFile(dbPath, []byte("db ruim"), 0o644)

	m := Marker{Target: target, BackupBinary: backup, DBSnapshot: snap, DBPath: dbPath}
	if err := Rollback(m); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if b, _ := os.ReadFile(target); string(b) != "BINARIO ANTIGO" {
		t.Errorf("target = %q, quero BINARIO ANTIGO", b)
	}

	rd, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()
	var x int
	if err := rd.QueryRow("SELECT x FROM t").Scan(&x); err != nil {
		t.Fatalf("DB não restaurado: %v", err)
	}
	if x != 7 {
		t.Errorf("x = %d, quero 7 (dado do snapshot)", x)
	}
}
