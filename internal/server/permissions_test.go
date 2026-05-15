package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

// setupRolesServer cria um servidor com banco, um admin e um viewer com acesso
// apenas a cam1 (não a cam2).
func setupRolesServer(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	database := openServerTestDB(t)

	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer", false)
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
	for _, cam := range cameras {
		if err := db.CreateCamera(database, cam, nil); err != nil {
			t.Fatalf("seed camera %q: %v", cam.ID, err)
		}
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", cameras, discardLogger(), nil).
		WithDB(database)

	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")
	viewerToken := loginAndGetToken(t, srv, "viewer_user", "viewerpw")
	return srv, adminToken, viewerToken
}

// --- /api/settings (admin-only) ---

func TestSettingsForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestSettingsAllowedForAdmin(t *testing.T) {
	srv, adminToken, _ := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- DELETE recording (admin-only) ---

func TestDeleteRecordingForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/cam1/recordings/20260511100000.mp4", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// --- GET /api/cameras (filtragem por viewer) ---

func TestGetCamerasFiltersForViewer(t *testing.T) {
	srv, _, viewerToken := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("expected 1 camera for viewer, got %d", len(list))
	}
	if list[0]["id"] != "cam1" {
		t.Errorf("expected cam1, got %v", list[0]["id"])
	}
}

func TestGetCamerasReturnsAllForAdmin(t *testing.T) {
	srv, adminToken, _ := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 cameras for admin, got %d", len(list))
	}
}

// --- /api/cameras/{id}/... (filtro por câmera) ---

func TestCameraEndpointForbiddenForViewerWithoutAccess(t *testing.T) {
	srv, _, viewerToken := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam2/recordings?date=2026-05-01&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCameraEndpointAllowedForViewerWithAccess(t *testing.T) {
	tmpDir := t.TempDir()
	database := openServerTestDB(t)

	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer", false)
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}
	if err := db.SetUserCameras(database, viewerID, []string{"cam1"}); err != nil {
		t.Fatalf("set user cameras: %v", err)
	}

	cameras := []config.CameraConfig{{ID: "cam1"}, {ID: "cam2"}}
	srv := server.NewServer(
		config.ServerConfig{RecordingsPath: tmpDir},
		"UTC", cameras, discardLogger(), nil,
	).WithDB(database)

	viewerToken := loginAndGetToken(t, srv, "viewer_user", "viewerpw")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/recordings?date=2026-05-01&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("expected non-403 for authorized camera, got %d", w.Code)
	}
}

// --- /stream/{id}/... ---

func TestStreamForbiddenForViewerWithoutAccess(t *testing.T) {
	tmpDir := t.TempDir()
	srv, _, viewerToken := setupRolesServer(t)
	_ = os.MkdirAll(filepath.Join(tmpDir, "cam2"), 0755)

	req := httptest.NewRequest(http.MethodGet, "/stream/cam2/index.m3u8", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestStreamNotForbiddenForViewerWithAccess(t *testing.T) {
	tmpDir := t.TempDir()
	database := openServerTestDB(t)

	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer", false)
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}
	if err := db.SetUserCameras(database, viewerID, []string{"cam1"}); err != nil {
		t.Fatalf("set user cameras: %v", err)
	}

	cameras := []config.CameraConfig{{ID: "cam1"}, {ID: "cam2"}}
	srv := server.NewServer(
		config.ServerConfig{SegmentsPath: tmpDir},
		"UTC", cameras, discardLogger(), nil,
	).WithDB(database)

	viewerToken := loginAndGetToken(t, srv, "viewer_user", "viewerpw")

	req := httptest.NewRequest(http.MethodGet, "/stream/cam1/index.m3u8", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("expected non-403 for authorized camera stream, got %d", w.Code)
	}
}

// --- /recordings/{id}/... ---

func TestRecordingsForbiddenForViewerWithoutAccess(t *testing.T) {
	srv, _, viewerToken := setupRolesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/recordings/cam2/2026/05/01/20260501100000.mp4", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRecordingsNotForbiddenForViewerWithAccess(t *testing.T) {
	tmpDir := t.TempDir()
	database := openServerTestDB(t)

	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	viewerID, err := db.CreateUser(database, "viewer_user", "viewerpw", "viewer", false)
	if err != nil {
		t.Fatalf("criar viewer: %v", err)
	}
	if err := db.SetUserCameras(database, viewerID, []string{"cam1"}); err != nil {
		t.Fatalf("set user cameras: %v", err)
	}

	cameras := []config.CameraConfig{{ID: "cam1"}, {ID: "cam2"}}
	srv := server.NewServer(
		config.ServerConfig{RecordingsPath: tmpDir},
		"UTC", cameras, discardLogger(), nil,
	).WithDB(database)

	viewerToken := loginAndGetToken(t, srv, "viewer_user", "viewerpw")

	req := httptest.NewRequest(http.MethodGet, "/recordings/cam1/2026/05/01/20260501100000.mp4", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("expected non-403 for authorized camera recordings, got %d", w.Code)
	}
}
