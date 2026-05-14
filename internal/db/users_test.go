package db_test

import (
	"path/filepath"
	"testing"

	"camera/internal/db"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestCreateAndGetUser(t *testing.T) {
	database := openTestDB(t)

	id, err := db.CreateUser(database, "alice", "senha123", "admin")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if id <= 0 {
		t.Fatalf("id inválido: %d", id)
	}

	u, err := db.GetUserByUsername(database, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username: got %q, want %q", u.Username, "alice")
	}
	if u.Role != "admin" {
		t.Errorf("role: got %q, want %q", u.Role, "admin")
	}
	if u.PasswordHash == "" {
		t.Error("password_hash vazio")
	}
	if u.PasswordHash == "senha123" {
		t.Error("password_hash deve ser hash bcrypt, não texto puro")
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	database := openTestDB(t)

	if _, err := db.CreateUser(database, "bob", "x", "viewer"); err != nil {
		t.Fatalf("primeiro CreateUser: %v", err)
	}
	_, err := db.CreateUser(database, "bob", "y", "viewer")
	if err == nil {
		t.Error("esperava erro por username duplicado")
	}
}

func TestListUsers(t *testing.T) {
	database := openTestDB(t)

	for _, u := range []struct{ name, role string }{
		{"alice", "admin"},
		{"bob", "viewer"},
		{"carol", "viewer"},
	} {
		if _, err := db.CreateUser(database, u.name, "x", u.role); err != nil {
			t.Fatalf("CreateUser %s: %v", u.name, err)
		}
	}

	users, err := db.ListUsers(database)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("esperava 3 usuários, got %d", len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	database := openTestDB(t)

	id, err := db.CreateUser(database, "dave", "senha", "viewer")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := db.UpdateUser(database, id, "dave2", "novasenha", "admin"); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	u, err := db.GetUserByID(database, id)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if u.Username != "dave2" {
		t.Errorf("username: got %q, want %q", u.Username, "dave2")
	}
	if u.Role != "admin" {
		t.Errorf("role: got %q, want %q", u.Role, "admin")
	}
}

func TestDeleteUser(t *testing.T) {
	database := openTestDB(t)

	id, err := db.CreateUser(database, "eve", "x", "viewer")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := db.DeleteUser(database, id); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err = db.GetUserByID(database, id)
	if err == nil {
		t.Error("esperava erro ao buscar usuário deletado")
	}
}

func TestSetAndGetUserCameras(t *testing.T) {
	database := openTestDB(t)

	id, err := db.CreateUser(database, "frank", "x", "viewer")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cameras := []string{"cam1", "cam2", "cam3"}
	if err := db.SetUserCameras(database, id, cameras); err != nil {
		t.Fatalf("SetUserCameras: %v", err)
	}

	got, err := db.GetUserCameras(database, id)
	if err != nil {
		t.Fatalf("GetUserCameras: %v", err)
	}
	if len(got) != len(cameras) {
		t.Errorf("esperava %d câmeras, got %d", len(cameras), len(got))
	}

	// substituir com lista menor
	if err := db.SetUserCameras(database, id, []string{"cam1"}); err != nil {
		t.Fatalf("SetUserCameras (substituição): %v", err)
	}
	got2, _ := db.GetUserCameras(database, id)
	if len(got2) != 1 {
		t.Errorf("após substituição: esperava 1, got %d", len(got2))
	}
}

func TestCheckPassword(t *testing.T) {
	database := openTestDB(t)

	if _, err := db.CreateUser(database, "grace", "minha-senha", "viewer"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	u, _ := db.GetUserByUsername(database, "grace")

	if !db.CheckPassword(u.PasswordHash, "minha-senha") {
		t.Error("CheckPassword deveria retornar true para senha correta")
	}
	if db.CheckPassword(u.PasswordHash, "errada") {
		t.Error("CheckPassword deveria retornar false para senha errada")
	}
}
