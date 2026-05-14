package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

// setupUsersServer cria um servidor com admin + viewer (cam1) e retorna tokens de ambos.
func setupUsersServer(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	database := openServerTestDB(t)

	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin"); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer")
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}
	if err := db.SetUserCameras(database, viewerID, []string{"cam1"}); err != nil {
		t.Fatalf("set user cameras: %v", err)
	}

	cameras := []config.CameraConfig{
		{ID: "cam1", RTSPURL: "rtsp://fake1"},
		{ID: "cam2", RTSPURL: "rtsp://fake2"},
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", cameras, discardLogger(), nil).
		WithDB(database)

	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")
	viewerToken := loginAndGetToken(t, srv, "viewer_user", "viewerpw")
	return srv, adminToken, viewerToken
}

// --- GET /api/users ---

func TestListUsers_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupUsersServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestListUsers_ReturnsAllForAdmin(t *testing.T) {
	srv, adminToken, _ := setupUsersServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 users, got %d", len(list))
	}
	// password_hash nunca deve vir na resposta
	for _, u := range list {
		if _, hasHash := u["password_hash"]; hasHash {
			t.Error("response must not include password_hash")
		}
	}
}

func TestListUsers_IncludesCamerasForViewer(t *testing.T) {
	srv, adminToken, _ := setupUsersServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)

	for _, u := range list {
		if u["username"] == "viewer_user" {
			cameras, ok := u["cameras"].([]any)
			if !ok || len(cameras) != 1 || cameras[0] != "cam1" {
				t.Errorf("expected viewer cameras=[cam1], got %v", u["cameras"])
			}
		}
	}
}

// --- POST /api/users ---

func TestCreateUser_Success(t *testing.T) {
	srv, adminToken, _ := setupUsersServer(t)

	body := `{"username":"newviewer","password":"pw1234","role":"viewer","cameras":["cam1","cam2"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["username"] != "newviewer" {
		t.Errorf("expected username=newviewer, got %v", resp["username"])
	}
	cameras, ok := resp["cameras"].([]any)
	if !ok || len(cameras) != 2 {
		t.Errorf("expected 2 cameras in response, got %v", resp["cameras"])
	}
}

func TestCreateUser_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupUsersServer(t)

	body := `{"username":"x","password":"y","role":"viewer"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	srv, adminToken, _ := setupUsersServer(t)

	body := `{"username":"admin_user","password":"pw","role":"viewer"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	srv, adminToken, _ := setupUsersServer(t)

	body := `{"username":"u","password":"p","role":"superuser"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- PUT /api/users/{id} ---

func TestUpdateUser_ChangeRole(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin"); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer")
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	body := fmt.Sprintf(`{"username":"viewer_user","role":"admin","password":""}`)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/users/%d", viewerID), bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["role"] != "admin" {
		t.Errorf("expected role=admin, got %v", resp["role"])
	}
}

func TestUpdateUser_SetCameras(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin"); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer")
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	body := fmt.Sprintf(`{"username":"viewer_user","role":"viewer","cameras":["cam1","cam2"]}`)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/users/%d", viewerID), bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	cameras, ok := resp["cameras"].([]any)
	if !ok || len(cameras) != 2 {
		t.Errorf("expected cameras=[cam1,cam2], got %v", resp["cameras"])
	}
}

func TestUpdateUser_NoPasswordChange(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin"); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer")
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	// Atualiza role sem fornecer nova senha
	body := fmt.Sprintf(`{"username":"viewer_user","role":"admin"}`)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/users/%d", viewerID), bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// A senha antiga ainda deve funcionar para login
	_ = loginAndGetToken(t, srv, "viewer_user", "viewerpw")
}

func TestUpdateUser_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupUsersServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/users/1", bytes.NewBufferString(`{"username":"x","role":"admin"}`))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// --- DELETE /api/users/{id} ---

func TestDeleteUser_Success(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin"); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	targetID, err := db.CreateUser(database, "todelete", "pw", "viewer")
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/users/%d", targetID), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	// Confirma que o usuário foi removido
	users, _ := db.ListUsers(database)
	for _, u := range users {
		if u.ID == targetID {
			t.Error("user should have been deleted")
		}
	}
}

func TestDeleteUser_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupUsersServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/1", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	srv, adminToken, _ := setupUsersServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/users/9999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
