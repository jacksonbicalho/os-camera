package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateCamera_PersistsMotionRTSPURL verifies the optional motion_rtsp_url
// field is accepted on create and returned in the camera DTO.
func TestCreateCamera_PersistsMotionRTSPURL(t *testing.T) {
	srv, adminToken, _, _, _ := setupCamerasServer(t)

	body := `{"name":"cam-sub","rtsp_url":"rtsp://fake/main","motion_rtsp_url":"rtsp://fake/sub"}`
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
	if resp["motion_rtsp_url"] != "rtsp://fake/sub" {
		t.Errorf("motion_rtsp_url = %v, want rtsp://fake/sub", resp["motion_rtsp_url"])
	}
}

// TestUpdateCamera_UpdatesMotionRTSPURL verifies the field can be set via update.
func TestUpdateCamera_UpdatesMotionRTSPURL(t *testing.T) {
	srv, adminToken, _, cam1ID, _ := setupCamerasServer(t)

	body := `{"name":"cam1","rtsp_url":"rtsp://fake1","motion_rtsp_url":"rtsp://fake1/sub"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/"+cam1ID, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Read it back via the settings list.
	getReq := httptest.NewRequest(http.MethodGet, "/api/settings/cameras", nil)
	getReq.Header.Set("Authorization", "Bearer "+adminToken)
	gw := httptest.NewRecorder()
	srv.ServeHTTP(gw, getReq)

	var list []map[string]any
	json.NewDecoder(gw.Body).Decode(&list)
	var found bool
	for _, c := range list {
		if c["id"] == cam1ID {
			found = true
			if c["motion_rtsp_url"] != "rtsp://fake1/sub" {
				t.Errorf("motion_rtsp_url = %v, want rtsp://fake1/sub", c["motion_rtsp_url"])
			}
		}
	}
	if !found {
		t.Fatalf("camera %s not found in list", cam1ID)
	}
}
