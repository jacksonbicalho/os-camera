package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/config"
	"camera/internal/release"
	"camera/internal/server"
)

type fakeChecker struct{ st release.Status }

func (f fakeChecker) Status() release.Status { return f.st }

func TestGetUpdates_WithChecker(t *testing.T) {
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, nil)
	srv.WithUpdateChecker(fakeChecker{st: release.Status{
		Current:         "v1.3.0-dev",
		Latest:          "v1.4.0-dev",
		NotesMD:         "### Novidades\n- algo",
		Image:           "jacksonbicalho/os-camera:1.4.0-dev",
		UpdateAvailable: true,
	}})
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/updates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var st release.Status
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !st.UpdateAvailable || st.Latest != "v1.4.0-dev" || st.Image == "" {
		t.Errorf("status inesperado: %+v", st)
	}
}

func TestGetUpdates_ApplyMode(t *testing.T) {
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, nil)
	srv.WithApplyMode("self-replace")
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/updates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ApplyMode string `json:"apply_mode"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ApplyMode != "self-replace" {
		t.Errorf("apply_mode = %q, quero self-replace", resp.ApplyMode)
	}
}

func TestGetUpdates_NoChecker(t *testing.T) {
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, nil)
	srv.WithVersion("v1.3.0-dev")
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/updates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var st release.Status
	json.Unmarshal(w.Body.Bytes(), &st)
	if st.UpdateAvailable {
		t.Error("sem checker não deveria haver update")
	}
	if st.Current != "v1.3.0-dev" {
		t.Errorf("Current = %q, quero v1.3.0-dev", st.Current)
	}
}
