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

func TestLoadParsesServerConfig(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: 8080
  segments_path: /tmp/hls
  username: master
  password: secret

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.SegmentsPath != "/tmp/hls" {
		t.Errorf("expected segments_path /tmp/hls, got %q", cfg.Server.SegmentsPath)
	}
	if cfg.Server.Username != "master" {
		t.Errorf("expected username %q, got %q", "master", cfg.Server.Username)
	}
	if cfg.Server.Password != "secret" {
		t.Errorf("expected password %q, got %q", "secret", cfg.Server.Password)
	}
}

func TestLoadParsesTimezoneAtRoot(t *testing.T) {
	path := writeTempYAML(t, `
timezone: America/Sao_Paulo

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "America/Sao_Paulo" {
		t.Errorf("expected timezone %q, got %q", "America/Sao_Paulo", cfg.Timezone)
	}
}

func TestLoadTimezoneDefaultsToUTC(t *testing.T) {
	path := writeTempYAML(t, `
cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "UTC" {
		t.Errorf("expected timezone UTC, got %q", cfg.Timezone)
	}
}

func TestLoadEnvVarOverridesTimezone(t *testing.T) {
	t.Setenv("CAMERA_TIMEZONE", "America/Manaus")

	path := writeTempYAML(t, `
timezone: America/Sao_Paulo

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "America/Manaus" {
		t.Errorf("expected America/Manaus, got %q", cfg.Timezone)
	}
}

func TestLoadParsesRetentionMinutes(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings
  retention_minutes: 120

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.RetentionMinutes != 120 {
		t.Errorf("expected RetentionMinutes=120, got %d", cfg.Storage.RetentionMinutes)
	}
}

func TestLoadParsesIntervalMinutes(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings
  interval_minutes: 5

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.IntervalMinutes != 5 {
		t.Errorf("expected IntervalMinutes=5, got %d", cfg.Storage.IntervalMinutes)
	}
}

func TestLoadEnvVarOverridesStoragePath(t *testing.T) {
	t.Setenv("CAMERA_STORAGE_PATH", "/tmp/from-env")

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

func TestEffectiveRetentionBothSet(t *testing.T) {
	s := config.StorageConfig{
		Retention: config.RetentionConfig{
			WithMotionMinutes:    10080,
			WithoutMotionMinutes: 1440,
		},
	}

	withMotion, withoutMotion := s.EffectiveRetention()

	if withMotion != 10080 {
		t.Errorf("expected withMotion=10080, got %d", withMotion)
	}
	if withoutMotion != 1440 {
		t.Errorf("expected withoutMotion=1440, got %d", withoutMotion)
	}
}

func TestEffectiveRetentionOnlyWithMotionSet(t *testing.T) {
	s := config.StorageConfig{
		Retention: config.RetentionConfig{
			WithMotionMinutes: 10080,
		},
	}

	withMotion, withoutMotion := s.EffectiveRetention()

	if withMotion != 10080 {
		t.Errorf("expected withMotion=10080, got %d", withMotion)
	}
	if withoutMotion != 10080 {
		t.Errorf("expected withoutMotion to inherit withMotion (10080), got %d", withoutMotion)
	}
}

func TestEffectiveRetentionOnlyWithoutMotionSet(t *testing.T) {
	s := config.StorageConfig{
		Retention: config.RetentionConfig{
			WithoutMotionMinutes: 1440,
		},
	}

	withMotion, withoutMotion := s.EffectiveRetention()

	if withMotion != 0 {
		t.Errorf("expected withMotion=0 (keep indefinitely), got %d", withMotion)
	}
	if withoutMotion != 1440 {
		t.Errorf("expected withoutMotion=1440, got %d", withoutMotion)
	}
}

func TestEffectiveRetentionFallsBackToLegacyRetentionMinutes(t *testing.T) {
	s := config.StorageConfig{
		RetentionMinutes: 120,
	}

	withMotion, withoutMotion := s.EffectiveRetention()

	if withMotion != 120 {
		t.Errorf("expected withMotion=120 from legacy fallback, got %d", withMotion)
	}
	if withoutMotion != 120 {
		t.Errorf("expected withoutMotion=120 from legacy fallback, got %d", withoutMotion)
	}
}

func TestLoadParsesRetentionBlock(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings
  retention:
    with_motion_minutes: 10080
    without_motion_minutes: 1440

cameras:
  - id: cam1
    rtsp_url: rtsp://localhost:8554/stream
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.Retention.WithMotionMinutes != 10080 {
		t.Errorf("expected WithMotionMinutes=10080, got %d", cfg.Storage.Retention.WithMotionMinutes)
	}
	if cfg.Storage.Retention.WithoutMotionMinutes != 1440 {
		t.Errorf("expected WithoutMotionMinutes=1440, got %d", cfg.Storage.Retention.WithoutMotionMinutes)
	}
}

func TestEffectiveMotionConfigFallsBackToGlobal(t *testing.T) {
	cam := config.CameraConfig{}
	global := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 4}

	got := cam.EffectiveMotionConfig(global)

	if !got.Enabled {
		t.Error("expected Enabled=true from global")
	}
	if got.Threshold != 0.05 {
		t.Errorf("expected Threshold=0.05, got %v", got.Threshold)
	}
	if got.FPS != 4 {
		t.Errorf("expected FPS=4, got %d", got.FPS)
	}
}

func TestEffectiveMotionConfigUsesPerCameraOverride(t *testing.T) {
	override := config.MotionConfig{Enabled: false, Threshold: 0.10, FPS: 1}
	cam := config.CameraConfig{Motion: &override}
	global := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 4}

	got := cam.EffectiveMotionConfig(global)

	if got.Enabled {
		t.Error("expected Enabled=false from camera override")
	}
	if got.Threshold != 0.10 {
		t.Errorf("expected Threshold=0.10, got %v", got.Threshold)
	}
	if got.FPS != 1 {
		t.Errorf("expected FPS=1, got %d", got.FPS)
	}
}
