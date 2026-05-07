package storage_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"camera/internal/storage"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func writeFile(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func writeFileWithSize(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{0}, size), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestClean_DeletesOldFiles(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "cam1", "2026", "01", "01", "20260101120000.mp4")
	writeFile(t, old, time.Now().Add(-31*time.Minute))

	storage.New(dir, 30, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("expected old file to be deleted")
	}
}

func TestClean_KeepsRecentFiles(t *testing.T) {
	dir := t.TempDir()
	recent := filepath.Join(dir, "cam1", "2026", "04", "30", "20260430120000.mp4")
	writeFile(t, recent, time.Now().Add(-1*time.Minute))

	storage.New(dir, 30, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(recent); err != nil {
		t.Errorf("expected recent file to exist: %v", err)
	}
}

func TestClean_DisabledWhenRetentionMinutesZero(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "cam1", "2024", "01", "01", "20240101000000.mp4")
	writeFile(t, old, time.Now().Add(-365*24*time.Hour))

	storage.New(dir, 0, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(old); err != nil {
		t.Errorf("expected file to exist when retention disabled: %v", err)
	}
}

func TestClean_IgnoresNonMp4Files(t *testing.T) {
	dir := t.TempDir()
	ts := filepath.Join(dir, "cam1", "2026", "01", "01", "001.ts")
	writeFile(t, ts, time.Now().Add(-31*time.Minute))

	storage.New(dir, 30, 0, 0, discardLogger()).Clean()

	if _, err := os.Stat(ts); err != nil {
		t.Errorf("expected non-mp4 file to be preserved: %v", err)
	}
}

func TestCheckSize_LogsWarnWhenAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	// 200 bytes total; maxSizeGB ~107 bytes, 70% threshold ~75 bytes → should warn
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file1.mp4"), 100)
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file2.mp4"), 100)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	const maxSizeGB = 1e-7 // ~107 bytes
	storage.New(dir, 0, maxSizeGB, 70, log).CheckSize()

	if !strings.Contains(buf.String(), "storage usage high") {
		t.Errorf("expected storage usage warning, got: %s", buf.String())
	}
}

func TestCheckSize_NoWarnWhenBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	// 50 bytes total; maxSizeGB ~107 bytes, 70% threshold ~75 bytes → no warn
	writeFileWithSize(t, filepath.Join(dir, "cam1", "file1.mp4"), 50)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	const maxSizeGB = 1e-7 // ~107 bytes
	storage.New(dir, 0, maxSizeGB, 70, log).CheckSize()

	if strings.Contains(buf.String(), "storage usage high") {
		t.Errorf("unexpected storage usage warning below threshold")
	}
}
