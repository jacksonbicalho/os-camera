package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func openServerTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestLoginWithDB_ValidCredentials(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "alice", "pass123", "admin"); err != nil {
		t.Fatalf("criar usuário: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).
		WithDB(database)

	body := `{"username":"alice","password":"pass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected non-empty token")
	}
}

func TestLoginWithDB_InvalidPassword(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "alice", "pass123", "admin"); err != nil {
		t.Fatalf("criar usuário: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).
		WithDB(database)

	body := `{"username":"alice","password":"errada"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLoginWithDB_UnknownUser(t *testing.T) {
	database := openServerTestDB(t)

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).
		WithDB(database)

	body := `{"username":"naoexiste","password":"qualquer"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestLoginWithDB_TokenAuthorizesProtectedEndpoint(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "alice", "pass123", "admin"); err != nil {
		t.Fatalf("criar usuário: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{}, discardLogger(), nil).
		WithDB(database)

	token := loginAndGetToken(t, srv, "alice", "pass123")
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
