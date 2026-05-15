package db_test

import (
	"testing"

	"camera/internal/config"
	"camera/internal/db"
)

func TestSeedFromBootstrap(t *testing.T) {
	cfg := config.Config{
		Debug:    false,
		Timezone: "America/Sao_Paulo",
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "senha-admin",
		},
		Server: config.ServerConfig{
			Port:         8080,
			SegmentsPath: "/tmp/hls",
		},
		Storage: config.StorageConfig{
			Path: "/tmp/recordings",
			Retention: config.RetentionConfig{
				WithMotionMinutes:    10080,
				WithoutMotionMinutes: 1440,
			},
			MaxSizeGB: 20,
		},
	}

	database := openTestDB(t)

	if err := db.SeedFromBootstrap(database, cfg); err != nil {
		t.Fatalf("SeedFromBootstrap: %v", err)
	}

	// usuário admin criado com must_change_password=true
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
	if !users[0].MustChangePassword {
		t.Error("esperava must_change_password=true no admin inicial")
	}

	// system_config deve ter server.port
	port, err := db.GetConfig(database, "server.port")
	if err != nil {
		t.Fatalf("GetConfig server.port: %v", err)
	}
	if port != "8080" {
		t.Errorf("server.port: got %q, want %q", port, "8080")
	}
}

func TestSeedFromBootstrap_DefaultsWhenEmpty(t *testing.T) {
	cfg := config.Config{}

	database := openTestDB(t)

	if err := db.SeedFromBootstrap(database, cfg); err != nil {
		t.Fatalf("SeedFromBootstrap: %v", err)
	}

	users, err := db.ListUsers(database)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("esperava 1 usuário, got %d", len(users))
	}
	if users[0].Username != "admin" {
		t.Errorf("username padrão: got %q, want %q", users[0].Username, "admin")
	}
	if !db.CheckPassword(users[0].PasswordHash, "changeme") {
		t.Error("senha padrão deveria ser 'changeme'")
	}
}
