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

func TestLoadParsesStoragePath(t *testing.T) {
	path := writeTempYAML(t, `
storage:
  path: /tmp/recordings
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Storage.Path != "/tmp/recordings" {
		t.Errorf("expected /tmp/recordings, got %q", cfg.Storage.Path)
	}
}

func TestLoadParsesDBPath(t *testing.T) {
	path := writeTempYAML(t, `
db_path: /data/camera.db
storage:
  path: /tmp/recordings
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBPath != "/data/camera.db" {
		t.Errorf("expected db_path /data/camera.db, got %q", cfg.DBPath)
	}
}

func TestLoadParsesAdminConfig(t *testing.T) {
	path := writeTempYAML(t, `
admin:
  username: master
  password: secret123
`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Admin.Username != "master" {
		t.Errorf("expected username %q, got %q", "master", cfg.Admin.Username)
	}
	if cfg.Admin.Password != "secret123" {
		t.Errorf("expected password %q, got %q", "secret123", cfg.Admin.Password)
	}
}

func TestLoadParsesLogConfig(t *testing.T) {
	path := writeTempYAML(t, `
log:
  output: file
  path: /var/log/camera
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

func TestLoadParsesLogRotationFields(t *testing.T) {
	path := writeTempYAML(t, `
log:
  output: file
  path: /var/log/camera
  max_size_mb: 25
  max_age_days: 7
  max_backups: 3
  compress: false
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.MaxSizeMBOrDefault() != 25 {
		t.Errorf("expected max_size_mb 25, got %d", cfg.Log.MaxSizeMBOrDefault())
	}
	if cfg.Log.MaxAgeDaysOrDefault() != 7 {
		t.Errorf("expected max_age_days 7, got %d", cfg.Log.MaxAgeDaysOrDefault())
	}
	if cfg.Log.MaxBackupsOrDefault() != 3 {
		t.Errorf("expected max_backups 3, got %d", cfg.Log.MaxBackupsOrDefault())
	}
	if cfg.Log.CompressOrDefault() {
		t.Error("expected compress false from file")
	}
}

func TestLogRotationDefaultsWhenAbsent(t *testing.T) {
	path := writeTempYAML(t, `
log:
  output: file
  path: /var/log/camera
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.MaxSizeMBOrDefault() != 50 {
		t.Errorf("expected default max_size_mb 50, got %d", cfg.Log.MaxSizeMBOrDefault())
	}
	if cfg.Log.MaxAgeDaysOrDefault() != 30 {
		t.Errorf("expected default max_age_days 30, got %d", cfg.Log.MaxAgeDaysOrDefault())
	}
	if cfg.Log.MaxBackupsOrDefault() != 10 {
		t.Errorf("expected default max_backups 10, got %d", cfg.Log.MaxBackupsOrDefault())
	}
	if !cfg.Log.CompressOrDefault() {
		t.Error("expected default compress true")
	}
}

func TestLogRotationZeroIsRespected(t *testing.T) {
	path := writeTempYAML(t, `
log:
  output: file
  path: /var/log/camera
  max_age_days: 0
  max_backups: 0
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Log.MaxAgeDaysOrDefault() != 0 {
		t.Errorf("expected explicit max_age_days 0 (unlimited), got %d", cfg.Log.MaxAgeDaysOrDefault())
	}
	if cfg.Log.MaxBackupsOrDefault() != 0 {
		t.Errorf("expected explicit max_backups 0 (unlimited), got %d", cfg.Log.MaxBackupsOrDefault())
	}
}

func TestDefaultReconnectIntervalIs2s(t *testing.T) {
	if config.DefaultReconnectInterval != 2*time.Second {
		t.Errorf("expected DefaultReconnectInterval 2s, got %v", config.DefaultReconnectInterval)
	}
	// Sem override por câmera, o efetivo cai no default.
	var cam config.CameraConfig
	if got := cam.EffectiveReconnectInterval(); got != 2*time.Second {
		t.Errorf("expected EffectiveReconnectInterval 2s without override, got %v", got)
	}
}

func TestLoadParsesDebugField(t *testing.T) {
	path := writeTempYAML(t, `debug: true`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Debug {
		t.Error("expected Debug to be true")
	}
}

func TestLoadDebugDefaultsToFalse(t *testing.T) {
	path := writeTempYAML(t, `storage:
  path: /tmp`)

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
}

func TestLoadParsesTimezoneAtRoot(t *testing.T) {
	path := writeTempYAML(t, `timezone: America/Sao_Paulo`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "America/Sao_Paulo" {
		t.Errorf("expected timezone %q, got %q", "America/Sao_Paulo", cfg.Timezone)
	}
}

func TestLoadTimezoneDefaultsToUTC(t *testing.T) {
	path := writeTempYAML(t, `storage:
  path: /tmp`)

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

	path := writeTempYAML(t, `timezone: America/Sao_Paulo`)

	cfg, err := config.Load(path)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "America/Manaus" {
		t.Errorf("expected America/Manaus, got %q", cfg.Timezone)
	}
}

func TestEffectiveMotionConfigReturnsZeroWhenNil(t *testing.T) {
	cam := config.CameraConfig{}

	got := cam.EffectiveMotionConfig()

	if got.Enabled {
		t.Error("expected Enabled=false when no motion config")
	}
}

func TestEffectiveMotionConfigUsesPerCameraOverride(t *testing.T) {
	override := config.MotionConfig{Enabled: true, Threshold: 0.10, FPS: 1}
	cam := config.CameraConfig{Motion: &override}

	got := cam.EffectiveMotionConfig()

	if !got.Enabled {
		t.Error("expected Enabled=true from camera override")
	}
	if got.Threshold != 0.10 {
		t.Errorf("expected Threshold=0.10, got %v", got.Threshold)
	}
	if got.FPS != 1 {
		t.Errorf("expected FPS=1, got %d", got.FPS)
	}
}

func TestEffectiveChunkDurationUsesConstantWhenNotSet(t *testing.T) {
	cam := config.CameraConfig{}

	if cam.EffectiveChunkDuration() != config.DefaultChunkDuration {
		t.Errorf("expected DefaultChunkDuration (%v), got %v", config.DefaultChunkDuration, cam.EffectiveChunkDuration())
	}
}

func TestEffectiveChunkDurationUsesExplicitValue(t *testing.T) {
	cam := config.CameraConfig{ChunkDuration: config.Duration(10 * time.Minute)}

	if cam.EffectiveChunkDuration() != 10*time.Minute {
		t.Errorf("expected 10m, got %v", cam.EffectiveChunkDuration())
	}
}

func TestLoadParsesJWTSecret(t *testing.T) {
	path := writeTempYAML(t, `
server:
  port: 8080
  jwt_secret: "my-super-secret-key-32chars-long!"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.JWTSecret != "my-super-secret-key-32chars-long!" {
		t.Errorf("expected jwt_secret from file, got %q", cfg.Server.JWTSecret)
	}
}

func TestLoadEnvVarOverridesJWTSecret(t *testing.T) {
	t.Setenv("CAMERA_SERVER_JWT_SECRET", "env-secret")

	path := writeTempYAML(t, `
server:
  port: 8080
  jwt_secret: "file-secret"
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.JWTSecret != "env-secret" {
		t.Errorf("expected env-secret from env var, got %q", cfg.Server.JWTSecret)
	}
}

func TestLoadEmptyJWTSecretWhenNotSet(t *testing.T) {
	path := writeTempYAML(t, `server:
  port: 8080
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.JWTSecret != "" {
		t.Errorf("expected empty JWTSecret when not configured, got %q", cfg.Server.JWTSecret)
	}
}
