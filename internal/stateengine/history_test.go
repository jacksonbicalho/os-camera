package stateengine_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"camera/internal/stateengine"
)

func TestSaveHistoryFrame(t *testing.T) {
	storage := t.TempDir()
	src := filepath.Join(storage, "tmp", "crop.jpg")
	if err := os.MkdirAll(filepath.Dir(src), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("jpeg-bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	ts := time.UnixMilli(1700000000000).UTC()
	servable, err := stateengine.SaveHistoryFrame(storage, 7, src, ts)
	if err != nil {
		t.Fatal(err)
	}
	if servable != "/recordings/state_history/7/1700000000000.jpg" {
		t.Fatalf("path servível inesperado: %q", servable)
	}
	// o arquivo durável existe com o mesmo conteúdo
	dst := filepath.Join(storage, "state_history", "7", "1700000000000.jpg")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("frame durável não gravado: %v", err)
	}
	if string(data) != "jpeg-bytes" {
		t.Fatalf("conteúdo do frame difere: %q", data)
	}
}
