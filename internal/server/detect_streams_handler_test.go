package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/ffprobe"
)

func postDetectStreams(t *testing.T, srv http.Handler, token, rtsp string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body := `{"rtsp_url":"` + rtsp + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/settings/cameras/detect-streams", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	return w, resp
}

func TestDetectStreams_RecommendsWebRTCForH264(t *testing.T) {
	exec := &fakeSubExecutor{out: []byte(`{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080}]}`)}
	srv, token := newDetectServer(t, ffprobe.NewProber(exec))

	w, resp := postDetectStreams(t, srv, token, "rtsp://cam/main")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if resp["codec"] != "h264" {
		t.Errorf("codec = %v, want h264", resp["codec"])
	}
	if resp["recommended"] != "webrtc" {
		t.Errorf("recommended = %v, want webrtc", resp["recommended"])
	}
	if resp["width"] != float64(1920) || resp["height"] != float64(1080) {
		t.Errorf("dims = %vx%v, want 1920x1080", resp["width"], resp["height"])
	}
}

func TestDetectStreams_RecommendsHLSForH265(t *testing.T) {
	exec := &fakeSubExecutor{out: []byte(`{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":1080}]}`)}
	srv, token := newDetectServer(t, ffprobe.NewProber(exec))

	_, resp := postDetectStreams(t, srv, token, "rtsp://cam/main")
	if resp["codec"] != "hevc" {
		t.Errorf("codec = %v, want hevc", resp["codec"])
	}
	if resp["recommended"] != "hls" {
		t.Errorf("recommended = %v, want hls", resp["recommended"])
	}
}
