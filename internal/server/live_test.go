package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pion/webrtc/v4"

	"camera/internal/server"
)

type fakeLivePublisher struct {
	answer string
	called bool
}

func (f *fakeLivePublisher) Negotiate(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	f.called = true
	return webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: f.answer}, nil
}

func TestWebRTCForbiddenForViewerWithoutAccess(t *testing.T) {
	srv, _, viewerToken := setupRolesServer(t)

	body, _ := json.Marshal(map[string]string{"sdp": "v=0\r\noffer"})
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/cam2/webrtc", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer without access, got %d", w.Code)
	}
}

func TestWebRTCConflictWhenNoPublisher(t *testing.T) {
	srv, adminToken, _ := setupRolesServer(t)

	body, _ := json.Marshal(map[string]string{"sdp": "v=0\r\noffer"})
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/cam1/webrtc", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 without publisher, got %d", w.Code)
	}
}

func TestWebRTCReturnsAnswerFromPublisher(t *testing.T) {
	srv, adminToken, _ := setupRolesServer(t)
	fake := &fakeLivePublisher{answer: "v=0\r\nanswer-sdp"}
	srv.(*server.Server).WithLivePublisher("cam1", fake)

	body, _ := json.Marshal(map[string]string{"sdp": "v=0\r\noffer-sdp"})
	req := httptest.NewRequest(http.MethodPost, "/api/cameras/cam1/webrtc", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		SDP string `json:"sdp"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SDP != fake.answer {
		t.Fatalf("expected answer sdp %q, got %q", fake.answer, resp.SDP)
	}
	if !fake.called {
		t.Fatal("expected Negotiate to be called")
	}
}
