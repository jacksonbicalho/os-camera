package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

// setupCamerasServer cria servidor com DB, um admin e duas câmeras iniciais.
func setupCamerasServer(t *testing.T) (http.Handler, string, string) {
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
		t.Fatalf("set cameras: %v", err)
	}

	for _, cam := range []config.CameraConfig{
		{ID: "cam1", RTSPURL: "rtsp://fake1"},
		{ID: "cam2", RTSPURL: "rtsp://fake2"},
	} {
		if err := db.CreateCamera(database, cam, nil); err != nil {
			t.Fatalf("criar câmera %s: %v", cam.ID, err)
		}
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

// --- GET /api/settings/cameras ---

func TestListSettingsCameras_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupCamerasServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestListSettingsCameras_ReturnsAll(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []map[string]any
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(list))
	}
	// rtsp_url deve vir na resposta (endpoint admin)
	if list[0]["rtsp_url"] == nil {
		t.Error("rtsp_url must be present in admin response")
	}
}

// --- POST /api/settings/cameras ---

func TestCreateCamera_Success(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	body := `{"id":"cam3","rtsp_url":"rtsp://fake3","chunk_duration":"2m"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != "cam3" {
		t.Errorf("expected id=cam3, got %v", resp["id"])
	}
}

func TestCreateCamera_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupCamerasServer(t)

	body := `{"id":"cam3","rtsp_url":"rtsp://fake3"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCreateCamera_DuplicateID(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	body := `{"id":"cam1","rtsp_url":"rtsp://outro"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateCamera_MissingFields(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	body := `{"id":"cam3"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- PUT /api/settings/cameras/{id} ---

func TestUpdateCamera_Success(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	body := `{"rtsp_url":"rtsp://updated","chunk_duration":"10m"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/cam1", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["rtsp_url"] != "rtsp://updated" {
		t.Errorf("expected rtsp_url=rtsp://updated, got %v", resp["rtsp_url"])
	}
}

func TestUpdateCamera_NotFound(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	body := `{"rtsp_url":"rtsp://x"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/inexistente", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- DELETE /api/settings/cameras/{id} ---

func TestDeleteCamera_Success(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	if err := db.CreateCamera(database, config.CameraConfig{ID: "todelete", RTSPURL: "rtsp://x"}, nil); err != nil {
		t.Fatalf("criar câmera: %v", err)
	}

	cameras := []config.CameraConfig{{ID: "todelete", RTSPURL: "rtsp://x"}}
	srv := server.NewServer(config.ServerConfig{}, "UTC", cameras, discardLogger(), nil).WithDB(database)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/cameras/todelete", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}

	cams, _ := db.ListCameras(database)
	for _, c := range cams {
		if c.ID == "todelete" {
			t.Error("camera should have been deleted from DB")
		}
	}
}

func TestDeleteCamera_NotFound(t *testing.T) {
	srv, adminToken, _ := setupCamerasServer(t)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/settings/cameras/inexistente"), nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteCamera_ForbiddenForViewer(t *testing.T) {
	srv, _, viewerToken := setupCamerasServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/cameras/cam1", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// --- callbacks de ciclo de vida ---

func TestCreateCamera_CallsOnCameraStart(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}

	var started []string
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).
		WithDB(database).
		WithCameraCallbacks(
			func(cam config.CameraConfig) { started = append(started, cam.ID) },
			nil,
		)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras",
		strings.NewReader(`{"id":"cam1","rtsp_url":"rtsp://fake"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	time.Sleep(50 * time.Millisecond)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if len(started) != 1 || started[0] != "cam1" {
		t.Errorf("expected onCameraStart(cam1), got %v", started)
	}
}

func TestDeleteCamera_CallsOnCameraStop(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	if err := db.CreateCamera(database, config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://x"}, nil); err != nil {
		t.Fatalf("criar câmera: %v", err)
	}

	var stopped []string
	srv := server.NewServer(config.ServerConfig{}, "UTC",
		[]config.CameraConfig{{ID: "cam1", RTSPURL: "rtsp://x"}}, discardLogger(), nil).
		WithDB(database).
		WithCameraCallbacks(nil, func(id string) { stopped = append(stopped, id) })
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/cameras/cam1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if len(stopped) != 1 || stopped[0] != "cam1" {
		t.Errorf("expected onCameraStop(cam1), got %v", stopped)
	}
}

func TestUpdateCamera_CallsStopThenStart(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	if err := db.CreateCamera(database, config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://old"}, nil); err != nil {
		t.Fatalf("criar câmera: %v", err)
	}

	var calls []string
	srv := server.NewServer(config.ServerConfig{}, "UTC",
		[]config.CameraConfig{{ID: "cam1", RTSPURL: "rtsp://old"}}, discardLogger(), nil).
		WithDB(database).
		WithCameraCallbacks(
			func(cam config.CameraConfig) { calls = append(calls, "start:"+cam.ID) },
			func(id string) { calls = append(calls, "stop:"+id) },
		)
	adminToken := loginAndGetToken(t, srv, "admin_user", "adminpw")

	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/cam1",
		strings.NewReader(`{"rtsp_url":"rtsp://new"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	time.Sleep(50 * time.Millisecond)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(calls) != 2 || calls[0] != "stop:cam1" || calls[1] != "start:cam1" {
		t.Errorf("expected [stop:cam1, start:cam1], got %v", calls)
	}
}
