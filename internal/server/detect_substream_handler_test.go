package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/ffprobe"
	"camera/internal/server"
)

// fakeSubExecutor returns canned ffprobe JSON for any URL, so the first derived
// substream candidate probes successfully.
type fakeSubExecutor struct{ out []byte }

func (f *fakeSubExecutor) Execute(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return f.out, nil
}

func newDetectServer(t *testing.T, prober *ffprobe.Prober) (http.Handler, string) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin_user", "adminpw", "admin", false); err != nil {
		t.Fatalf("criar admin: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	if prober != nil {
		srv = srv.WithProber(prober)
	}
	return srv, loginAndGetToken(t, srv, "admin_user", "adminpw")
}

func postDetect(t *testing.T, srv http.Handler, token, rtsp string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body := `{"rtsp_url":"` + rtsp + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras/detect-substream", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	return w, resp
}

func TestDetectSubstream_ReturnsVerifiedCandidate(t *testing.T) {
	exec := &fakeSubExecutor{out: []byte(`{"streams":[{"codec_type":"video","codec_name":"h264","width":640,"height":480}]}`)}
	srv, token := newDetectServer(t, ffprobe.NewProber(exec))

	w, resp := postDetect(t, srv, token, "rtsp://cam/realmonitor?channel=1&subtype=0")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if resp["motion_rtsp_url"] != "rtsp://cam/realmonitor?channel=1&subtype=1" {
		t.Errorf("motion_rtsp_url = %v, want subtype=1 variant", resp["motion_rtsp_url"])
	}
	if resp["width"] != float64(640) || resp["height"] != float64(480) {
		t.Errorf("dims = %vx%v, want 640x480", resp["width"], resp["height"])
	}
}

func TestDetectSubstream_EmptyWhenNoCandidate(t *testing.T) {
	exec := &fakeSubExecutor{out: []byte(`{"streams":[{"codec_type":"video","width":640,"height":480}]}`)}
	srv, token := newDetectServer(t, ffprobe.NewProber(exec))

	// No derivable convention → empty result, still 200 (not an error).
	w, resp := postDetect(t, srv, token, "rtsp://cam/Streaming/Channels/101")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if resp["motion_rtsp_url"] != "" {
		t.Errorf("motion_rtsp_url = %v, want empty", resp["motion_rtsp_url"])
	}
}
