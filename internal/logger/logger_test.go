package logger_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
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

func TestNewLoggerWritesToFile(t *testing.T) {
	dir := t.TempDir()

	_, err := logger.New(logger.Options{Output: "file", Path: dir})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "camera.log")); os.IsNotExist(err) {
		t.Error("expected camera.log to be created in the log directory")
	}
}

func TestNewLoggerReturnsErrorForInvalidOutput(t *testing.T) {
	_, err := logger.New(logger.Options{Output: "invalid"})

	if err == nil {
		t.Error("expected error for invalid output type")
	}
}
