package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindChunkForTime(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cam1", "2026/06/16")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260616180337.mp4", "20260616180407.mp4", "20260616180437.mp4"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// 18:04:20 cai no chunk que começa 18:04:07 (offset ~13s)
	ts := time.Date(2026, 6, 16, 18, 4, 20, 0, time.UTC)
	path, off, ok := findChunkForTime(root, "cam1", ts)
	if !ok {
		t.Fatal("esperava achar o chunk")
	}
	if filepath.Base(path) != "20260616180407.mp4" {
		t.Fatalf("chunk errado: %s", filepath.Base(path))
	}
	if off < 12 || off > 14 {
		t.Fatalf("offset ~13s esperado, got %v", off)
	}
}

func TestFindChunkForTimeBeforeFirst(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cam1", "2026/06/16")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "20260616180407.mp4"), []byte("x"), 0644)
	// antes do primeiro chunk → não encontrado
	ts := time.Date(2026, 6, 16, 18, 0, 0, 0, time.UTC)
	if _, _, ok := findChunkForTime(root, "cam1", ts); ok {
		t.Fatal("não deveria achar chunk antes do primeiro")
	}
}

func TestFindChunkForTimeNoDir(t *testing.T) {
	if _, _, ok := findChunkForTime(t.TempDir(), "cam1", time.Now()); ok {
		t.Fatal("esperava não encontrado sem gravações")
	}
}
