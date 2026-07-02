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

// TestSettings_ReturnsMaskedMotionRTSPURL verifies GET /api/settings (the endpoint
// the camera edit form reads via useSettings) exposes motion_rtsp_url with the
// password masked — mirroring how it already masks the main rtsp_url. Without it the
// motion substream field loads empty in the edit form.
func TestSettings_ReturnsMaskedMotionRTSPURL(t *testing.T) {
	srv, adminToken, _, cam1ID, _ := setupCamerasServer(t)

	body := `{"name":"cam1","rtsp_url":"rtsp://admin:s3cr3t@192.168.1.16:554/main","motion_rtsp_url":"rtsp://admin:s3cr3t@192.168.1.16:554/sub"}`
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
			if c["motion_rtsp_url"] != "rtsp://admin:xxxxx@192.168.1.16:554/sub" {
				t.Errorf("motion_rtsp_url = %v, want masked rtsp://admin:xxxxx@192.168.1.16:554/sub", c["motion_rtsp_url"])
			}
		}
	}
	if !found {
		t.Fatalf("camera %s not found in /api/settings", cam1ID)
	}
}

// TestUpdateCamera_PreservesMotionPasswordOnMaskedResubmit verifies that when the
// form resubmits the motion_rtsp_url with the masked password ("xxxxx", as it would
// after loading the masked value from /api/settings), the update restores the real
// password from the stored record instead of persisting "xxxxx".
func TestUpdateCamera_PreservesMotionPasswordOnMaskedResubmit(t *testing.T) {
	srv, adminToken, _, cam1ID, _ := setupCamerasServer(t)

	// Store the real motion password first.
	setup := `{"name":"cam1","rtsp_url":"rtsp://admin:s3cr3t@192.168.1.16:554/main","motion_rtsp_url":"rtsp://admin:s3cr3t@192.168.1.16:554/sub"}`
	req := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/"+cam1ID, bytes.NewBufferString(setup))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	// Resubmit with masked passwords, as the form does.
	masked := `{"name":"cam1","rtsp_url":"rtsp://admin:xxxxx@192.168.1.16:554/main","motion_rtsp_url":"rtsp://admin:xxxxx@192.168.1.16:554/sub"}`
	req2 := httptest.NewRequest(http.MethodPut, "/api/settings/cameras/"+cam1ID, bytes.NewBufferString(masked))
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// Read back the raw (unmasked) value via the admin cameras list.
	getReq := httptest.NewRequest(http.MethodGet, "/api/settings/cameras", nil)
	getReq.Header.Set("Authorization", "Bearer "+adminToken)
	gw := httptest.NewRecorder()
	srv.ServeHTTP(gw, getReq)

	var list []map[string]any
	json.NewDecoder(gw.Body).Decode(&list)
	for _, c := range list {
		if c["id"] == cam1ID {
			if c["motion_rtsp_url"] != "rtsp://admin:s3cr3t@192.168.1.16:554/sub" {
				t.Errorf("motion_rtsp_url = %v, want real password preserved", c["motion_rtsp_url"])
			}
		}
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
