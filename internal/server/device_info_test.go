package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/deviceinfo"
	"camera/internal/server"
)

// fakeCollector returns canned values without touching the network.
type fakeCollector struct{ values map[string]string }

func (fakeCollector) Name() string                                   { return "fake" }
func (fakeCollector) Detect(context.Context, deviceinfo.Target) bool { return true }
func (f fakeCollector) Collect(context.Context, deviceinfo.Target) (map[string]string, error) {
	return f.values, nil
}

func setupDeviceInfoServer(t *testing.T) (*server.Server, *db.DB, string, string) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://admin:pw@192.168.1.29:554/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("create camera: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, database, token, cam.ID
}

func TestGetDeviceInfo_NotCaptured(t *testing.T) {
	srv, _, token, id := setupDeviceInfoServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+id+"/device-info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before capture, got %d", w.Code)
	}
}

func TestGetDeviceInfo_ReturnsSaved(t *testing.T) {
	srv, database, token, id := setupDeviceInfoServer(t)
	if err := db.SaveDeviceInfo(database, id, map[string]string{
		"collector": "dahua", "model": "iM5", "ntp.enabled": "true",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+id+"/device-info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		CollectedAt string            `json:"collected_at"`
		Values      map[string]string `json:"values"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Values["model"] != "iM5" || resp.Values["ntp.enabled"] != "true" {
		t.Errorf("unexpected values: %v", resp.Values)
	}
}

func TestRefreshDeviceInfo_CollectsAndPersists(t *testing.T) {
	srv, database, token, id := setupDeviceInfoServer(t)
	srv.WithDeviceInfoCollectors(fakeCollector{values: map[string]string{
		"collector": "fake", "model": "X9",
	}})

	req := httptest.NewRequest(http.MethodPost, "/api/cameras/"+id+"/device-info/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Values map[string]string `json:"values"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Values["model"] != "X9" {
		t.Errorf("refresh response model = %v, want X9", resp.Values["model"])
	}

	got, _, ok, err := db.GetDeviceInfo(database, id)
	if err != nil || !ok {
		t.Fatalf("GetDeviceInfo after refresh: ok=%v err=%v", ok, err)
	}
	if got["model"] != "X9" {
		t.Errorf("persisted model = %q, want X9", got["model"])
	}
}

func TestCaptureMissingDeviceInfo(t *testing.T) {
	database := openServerTestDB(t)
	cam1 := config.CameraConfig{ID: "cam1", Name: "A", RTSPURL: "rtsp://admin:pw@1.1.1.1/"}
	cam2 := config.CameraConfig{ID: "cam2", Name: "B", RTSPURL: "rtsp://admin:pw@2.2.2.2/"}
	for _, c := range []config.CameraConfig{cam1, cam2} {
		if _, err := db.CreateCamera(database, c, nil); err != nil {
			t.Fatalf("create camera %s: %v", c.ID, err)
		}
	}
	// cam1 already has device info; cam2 has none.
	if err := db.SaveDeviceInfo(database, "cam1", map[string]string{"collector": "old", "model": "existing"}); err != nil {
		t.Fatalf("seed cam1: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam1, cam2}, discardLogger(), nil).
		WithDB(database).
		WithDeviceInfoCollectors(fakeCollector{values: map[string]string{"collector": "fake", "model": "NEW"}})

	srv.CaptureMissingDeviceInfo(context.Background())

	// cam2 gets captured.
	got2, _, ok2, _ := db.GetDeviceInfo(database, "cam2")
	if !ok2 || got2["model"] != "NEW" {
		t.Errorf("cam2 not captured: ok=%v values=%v", ok2, got2)
	}
	// cam1 is left untouched (already had info).
	got1, _, _, _ := db.GetDeviceInfo(database, "cam1")
	if got1["model"] != "existing" {
		t.Errorf("cam1 overwritten: %v", got1)
	}
}
