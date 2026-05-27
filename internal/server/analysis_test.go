package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func setupWithCamera(t *testing.T) (http.Handler, string) {
	t.Helper()
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil)
	cameras := []config.CameraConfig{{ID: "cam1", RTSPURL: "rtsp://fake/cam1"}}
	srv = withTestUsersAndCameras(t, srv, cameras)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, token
}

func TestGetAnalysisConfig_Default(t *testing.T) {
	srv, token := setupDrivesServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/analysis", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var cfg db.VideoAnalysisConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Enabled {
		t.Error("default enabled should be false")
	}
	if cfg.Model != "yolov8n" {
		t.Errorf("default model = %q, want yolov8n", cfg.Model)
	}
}

func TestUpdateAnalysisConfig(t *testing.T) {
	srv, token := setupDrivesServer(t)

	body := `{"enabled":true,"service_url":"http://yolo:8000","model":"yolov8s","confidence_threshold":0.5}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/analysis", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/settings/analysis", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var cfg db.VideoAnalysisConfig
	json.Unmarshal(w2.Body.Bytes(), &cfg)
	if !cfg.Enabled {
		t.Error("enabled should be true after update")
	}
	if cfg.ServiceURL != "http://yolo:8000" {
		t.Errorf("ServiceURL = %q, want http://yolo:8000", cfg.ServiceURL)
	}
}

func TestGetCameraAnalysisConfig_Default(t *testing.T) {
	srv, token := setupWithCamera(t)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/cameras/cam1/analysis", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["enabled"] != true {
		t.Errorf("default per-camera enabled = %v, want true", result["enabled"])
	}
}

func TestUpdateCameraAnalysisConfig(t *testing.T) {
	srv, token := setupWithCamera(t)

	body := `{"enabled":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/cam1/analysis", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/settings/cameras/cam1/analysis", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var result map[string]any
	json.Unmarshal(w2.Body.Bytes(), &result)
	if result["enabled"] != false {
		t.Errorf("enabled should be false after update, got %v", result["enabled"])
	}
}
