package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateCamera_PersistsLiveTransport verifies live_transport is accepted on
// create and returned in the DTO.
func TestCreateCamera_PersistsLiveTransport(t *testing.T) {
	srv, adminToken, _, _, _ := setupCamerasServer(t)

	body := `{"name":"cam-lt","rtsp_url":"rtsp://fake/main","live_transport":"hls"}`
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
	if resp["live_transport"] != "hls" {
		t.Errorf("live_transport = %v, want hls", resp["live_transport"])
	}
}

// TestCreateCamera_DefaultsLiveTransportAuto verifies an omitted or invalid
// value normalizes to "auto".
func TestCreateCamera_DefaultsLiveTransportAuto(t *testing.T) {
	srv, adminToken, _, _, _ := setupCamerasServer(t)

	for _, body := range []string{
		`{"name":"cam-a","rtsp_url":"rtsp://fake/a"}`,
		`{"name":"cam-b","rtsp_url":"rtsp://fake/b","live_transport":"bogus"}`,
	} {
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
		if resp["live_transport"] != "auto" {
			t.Errorf("live_transport = %v, want auto (body=%s)", resp["live_transport"], body)
		}
	}
}

// TestUpdateCamera_UpdatesLiveTransport verifies the field can be set via update
// and comes back in GET /api/settings.
func TestUpdateCamera_UpdatesLiveTransport(t *testing.T) {
	srv, adminToken, _, cam1ID, _ := setupCamerasServer(t)

	body := `{"name":"cam1","rtsp_url":"rtsp://fake1","live_transport":"webrtc"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/"+cam1ID, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getReq.Header.Set("Authorization", "Bearer "+adminToken)
	gw := httptest.NewRecorder()
	srv.ServeHTTP(gw, getReq)

	var resp struct {
		Cameras []map[string]any `json:"cameras"`
	}
	json.NewDecoder(gw.Body).Decode(&resp)
	var found bool
	for _, c := range resp.Cameras {
		if c["id"] == cam1ID {
			found = true
			if c["live_transport"] != "webrtc" {
				t.Errorf("live_transport = %v, want webrtc", c["live_transport"])
			}
		}
	}
	if !found {
		t.Fatalf("camera %s not found in /api/settings", cam1ID)
	}
}
