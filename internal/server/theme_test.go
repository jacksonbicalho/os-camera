package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func themeServer(t *testing.T) (*server.Server, string) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "u1", "pw", "viewer", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "u1", "pw")
	return srv, token
}

func getTheme(t *testing.T, srv http.Handler, token string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/me/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET preferences: expected 200, got %d", w.Code)
	}
	var r struct {
		Theme string `json:"theme"`
	}
	if err := json.NewDecoder(w.Body).Decode(&r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return r.Theme
}

func putTheme(t *testing.T, srv http.Handler, token, theme string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/me/preferences", strings.NewReader(`{"theme":"`+theme+`"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w.Code
}

func TestPreferences_DefaultThemeIsDark(t *testing.T) {
	srv, token := themeServer(t)
	if th := getTheme(t, srv, token); th != "dark" {
		t.Errorf("expected default theme 'dark', got %q", th)
	}
}

func TestPreferences_SetValidTheme(t *testing.T) {
	srv, token := themeServer(t)
	if code := putTheme(t, srv, token, "moderno"); code != http.StatusNoContent && code != http.StatusOK {
		t.Fatalf("PUT moderno: expected 200/204, got %d", code)
	}
	if th := getTheme(t, srv, token); th != "moderno" {
		t.Errorf("expected 'moderno' after PUT, got %q", th)
	}
}

func TestPreferences_AcceptsSystemTheme(t *testing.T) {
	srv, token := themeServer(t)
	if code := putTheme(t, srv, token, "system"); code != http.StatusNoContent && code != http.StatusOK {
		t.Fatalf("PUT system: expected 200/204, got %d", code)
	}
	if th := getTheme(t, srv, token); th != "system" {
		t.Errorf("expected 'system' after PUT, got %q", th)
	}
}

func TestPreferences_RejectsInvalidTheme(t *testing.T) {
	srv, token := themeServer(t)
	if code := putTheme(t, srv, token, "rainbow"); code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid theme, got %d", code)
	}
}

func TestPreferences_RequiresAuth(t *testing.T) {
	srv, _ := themeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/me/preferences", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", w.Code)
	}
}
