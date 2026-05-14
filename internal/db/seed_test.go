package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"camera/internal/db"
)

const testYAML = `
timezone: America/Sao_Paulo
server:
  port: 8080
  segments_path: /tmp/hls
  username: admin
  password: senha-admin
  jwt_secret: ""
storage:
  path: /tmp/recordings
  retention:
    with_motion_minutes: 10080
    without_motion_minutes: 1440
  max_size_gb: 20
motion:
  enabled: true
  threshold: 0.03
  fps: 3
  cooldown_seconds: 45
cameras:
  - id: entrada
    rtsp_url: rtsp://192.168.1.10:554/stream
    display_order: 0
  - id: garagem
    rtsp_url: rtsp://192.168.1.11:554/stream
    display_order: 1
    motion:
      enabled: false
      threshold: 0.05
      fps: 2
      cooldown_seconds: 30
`

func TestSeedFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "camera.yaml")
	if err := os.WriteFile(yamlPath, []byte(testYAML), 0644); err != nil {
		t.Fatalf("escrever yaml: %v", err)
	}

	database := openTestDB(t)

	if err := db.SeedFromYAML(database, yamlPath); err != nil {
		t.Fatalf("SeedFromYAML: %v", err)
	}

	// câmeras
	cams, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(cams) != 2 {
		t.Errorf("esperava 2 câmeras, got %d", len(cams))
	}

	// garagem deve ter motion config própria
	garagem, err := db.GetCamera(database, "garagem")
	if err != nil {
		t.Fatalf("GetCamera garagem: %v", err)
	}
	if garagem.Motion == nil {
		t.Fatal("garagem.motion é nil")
	}
	if garagem.Motion.Enabled {
		t.Error("garagem.motion.enabled deveria ser false")
	}

	// config do sistema
	port, err := db.GetConfig(database, "server.port")
	if err != nil {
		t.Fatalf("GetConfig server.port: %v", err)
	}
	if port != "8080" {
		t.Errorf("server.port: got %q, want %q", port, "8080")
	}

	// usuário admin criado
	users, err := db.ListUsers(database)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("esperava 1 usuário, got %d", len(users))
	}
	if users[0].Username != "admin" {
		t.Errorf("username: got %q, want %q", users[0].Username, "admin")
	}
	if users[0].Role != "admin" {
		t.Errorf("role: got %q, want %q", users[0].Role, "admin")
	}
	if !db.CheckPassword(users[0].PasswordHash, "senha-admin") {
		t.Error("senha do admin não confere")
	}

	// yaml deve permanecer intacto (IsNew já evita re-seed)
	if _, err := os.Stat(yamlPath); err != nil {
		t.Errorf("camera.yaml deveria permanecer após seed: %v", err)
	}
}

func TestSeedFromYAML_MissingFile(t *testing.T) {
	database := openTestDB(t)

	err := db.SeedFromYAML(database, "/caminho/inexistente/camera.yaml")
	if err == nil {
		t.Error("esperava erro para arquivo inexistente")
	}
}
