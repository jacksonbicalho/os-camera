package config_test

import (
	"os"
	"testing"
	"time"

	"camera/internal/config"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoadReturnsErrorWhenFileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/camera.yaml")

	if err == nil {
		t.Error("expected error when config file does not exist")
	}
}

func TestLoadParsesCamera(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings

cameras:
  - id: entrada
    rtsp_url: rtsp://192.168.1.10:554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Cameras) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(cfg.Cameras))
	}
	if cfg.Cameras[0].ID != "entrada" {
		t.Errorf("expected id %q, got %q", "entrada", cfg.Cameras[0].ID)
	}
	if cfg.Cameras[0].RTSPURL != "rtsp://192.168.1.10:554/stream" {
		t.Errorf("expected rtsp_url %q, got %q", "rtsp://192.168.1.10:554/stream", cfg.Cameras[0].RTSPURL)
	}
}

func TestLoadParsesMultipleCameras(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings

cameras:
  - id: entrada
    rtsp_url: rtsp://192.168.1.10:554/stream
  - id: quintal
    rtsp_url: rtsp://192.168.1.11:554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Cameras) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(cfg.Cameras))
	}
	if cfg.Cameras[1].ID != "quintal" {
		t.Errorf("expected id %q, got %q", "quintal", cfg.Cameras[1].ID)
	}
	if cfg.Cameras[1].RTSPURL != "rtsp://192.168.1.11:554/stream" {
		t.Errorf("expected rtsp_url %q, got %q", "rtsp://192.168.1.11:554/stream", cfg.Cameras[1].RTSPURL)
	}
}

func TestLoadParsesDefaultChunkDuration(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings

defaults:
  chunk_duration: 5m

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if time.Duration(cfg.Defaults.ChunkDuration) != 5*time.Minute {
		t.Errorf("expected 5m, got %v", cfg.Defaults.ChunkDuration)
	}
}

func TestLoadCameraOverridesChunkDuration(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings

defaults:
  chunk_duration: 5m

cameras:
  - id: entrada
    rtsp_url: rtsp://192.168.1.10:554/stream
    chunk_duration: 10m
  - id: quintal
    rtsp_url: rtsp://192.168.1.11:554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cameras[0].EffectiveChunkDuration(cfg.Defaults) != 10*time.Minute {
		t.Errorf("expected 10m for camera with explicit duration, got %v", cfg.Cameras[0].EffectiveChunkDuration(cfg.Defaults))
	}
	if cfg.Cameras[1].EffectiveChunkDuration(cfg.Defaults) != 5*time.Minute {
		t.Errorf("expected 5m for camera falling back to default, got %v", cfg.Cameras[1].EffectiveChunkDuration(cfg.Defaults))
	}
}

func TestLoadParsesStoragePath(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.Path != "/tmp/recordings" {
		t.Errorf("expected /tmp/recordings, got %q", cfg.Storage.Path)
	}
}

func TestLoadParsesLogConfig(t *testing.T) {
	path := writeTempYAML(t, `
log:
  output: file
  path: /var/log/camera

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.Output != "file" {
		t.Errorf("expected output %q, got %q", "file", cfg.Log.Output)
	}
	if cfg.Log.Path != "/var/log/camera" {
		t.Errorf("expected path %q, got %q", "/var/log/camera", cfg.Log.Path)
	}
}

func TestLoadParsesDebugField(t *testing.T) {
	path := writeTempYAML(t, `
debug: true

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Debug {
		t.Error("expected Debug to be true")
	}
}

func TestLoadDebugDefaultsToFalse(t *testing.T) {
	path := writeTempYAML(t, `
cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Debug {
		t.Error("expected Debug to default to false")
	}
}

func TestLoadEnvVarOverridesStoragePath(t *testing.T) {
	t.Setenv("STORAGE_PATH", "/tmp/from-env")

	path := writeTempYAML(t, `
storage:
  path: /tmp/from-file

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.Path != "/tmp/from-env" {
		t.Errorf("expected /tmp/from-env, got %q", cfg.Storage.Path)
	}
}
