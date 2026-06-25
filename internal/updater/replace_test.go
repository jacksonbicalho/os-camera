package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplace(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "camera")
	src := filepath.Join(dir, "camera.new")
	backup := filepath.Join(dir, "camera.old")

	if err := os.WriteFile(target, []byte("VERSAO ANTIGA"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("VERSAO NOVA"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(src, target, backup); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	if b, _ := os.ReadFile(target); string(b) != "VERSAO NOVA" {
		t.Errorf("target = %q, quero VERSAO NOVA", b)
	}
	if b, _ := os.ReadFile(backup); string(b) != "VERSAO ANTIGA" {
		t.Errorf("backup = %q, quero VERSAO ANTIGA", b)
	}
}
