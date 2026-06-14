package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

// setupStreamCacheServer wires a server whose SegmentsPath holds a playlist and
// a segment for an accessible camera, returning an admin token and the camera id.
func setupStreamCacheServer(t *testing.T) (*server.Server, string, string) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://admin:pw@192.168.1.29:554/"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("create camera: %v", err)
	}

	segDir := filepath.Join(t.TempDir(), cam.ID)
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatalf("mkdir segments: %v", err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "index.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatalf("write playlist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(segDir, "000000.ts"), []byte("ts-bytes"), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}

	cfg := config.ServerConfig{SegmentsPath: filepath.Dir(segDir)}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, token, cam.ID
}

func TestStreamPlaylistIsNotCacheable(t *testing.T) {
	srv, token, id := setupStreamCacheServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stream/"+id+"/index.m3u8", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	cc := w.Header().Get("Cache-Control")
	if !strings.Contains(cc, "no-cache") {
		t.Fatalf("playlist must not be cacheable; Cache-Control=%q", cc)
	}
}

func TestStreamSegmentStaysCacheable(t *testing.T) {
	srv, token, id := setupStreamCacheServer(t)

	req := httptest.NewRequest(http.MethodGet, "/stream/"+id+"/000000.ts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if cc := w.Header().Get("Cache-Control"); strings.Contains(cc, "no-cache") {
		t.Fatalf("segments should remain cacheable; got Cache-Control=%q", cc)
	}
}
