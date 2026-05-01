package logger_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"camera/internal/logger"
)

func TestNewLoggerUsesInfoLevelByDefault(t *testing.T) {
	log, err := logger.New(logger.Options{Output: "stdout"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if log.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected DEBUG to be disabled when debug=false")
	}
	if !log.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("expected INFO to be enabled when debug=false")
	}
}

func TestNewLoggerEnablesDebugLevel(t *testing.T) {
	log, err := logger.New(logger.Options{Output: "stdout", Debug: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !log.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected DEBUG to be enabled when debug=true")
	}
}

func TestNewLoggerFileOutputCreatesLevelFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := logger.New(logger.Options{Output: "file", Path: dir, Debug: true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"debug.log", "info.log", "warn.log", "error.log"} {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			t.Errorf("expected %s to be created", name)
		}
	}
}

func TestNewLoggerWritesInfoOnlyToInfoLog(t *testing.T) {
	dir := t.TempDir()
	log, err := logger.New(logger.Options{Output: "file", Path: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Info("hello info")

	assertFileContains(t, filepath.Join(dir, "info.log"), "hello info")
	assertFileNotContains(t, filepath.Join(dir, "error.log"), "hello info")
}

func TestNewLoggerWritesErrorOnlyToErrorLog(t *testing.T) {
	dir := t.TempDir()
	log, err := logger.New(logger.Options{Output: "file", Path: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	log.Error("something failed")

	assertFileContains(t, filepath.Join(dir, "error.log"), "something failed")
	assertFileNotContains(t, filepath.Join(dir, "info.log"), "something failed")
}

func TestNewLoggerReturnsErrorForInvalidOutput(t *testing.T) {
	_, err := logger.New(logger.Options{Output: "invalid"})

	if err == nil {
		t.Error("expected error for invalid output type")
	}
}

func assertFileContains(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if !strings.Contains(string(data), substr) {
		t.Errorf("expected %s to contain %q", path, substr)
	}
}

func assertFileNotContains(t *testing.T, path, substr string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if strings.Contains(string(data), substr) {
		t.Errorf("expected %s NOT to contain %q", path, substr)
	}
}
