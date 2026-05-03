package server_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"camera/internal/config"
	"camera/internal/server"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoginReturnsTokenForValidCredentials(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	body := `{"username":"master","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected non-empty token in response")
	}
}

func loginAndGetToken(t *testing.T, srv http.Handler, username, password string) string {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	return resp["token"]
}

func TestLoginReturnsUnauthorizedForInvalidCredentials(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	body := `{"username":"master","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetCamerasReturnsList(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameras := []config.CameraConfig{
		{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"},
		{ID: "quintal", RTSPURL: "rtsp://192.168.1.11:554/stream"},
	}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []map[string]string
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(list))
	}
	if list[0]["id"] != "entrada" {
		t.Errorf("expected first camera id %q, got %q", "entrada", list[0]["id"])
	}
}

func TestGetCamerasRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestStreamServesHLSPlaylist(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"
	segDir := filepath.Join(tmpDir, cameraID)
	if err := os.MkdirAll(segDir, 0755); err != nil {
		t.Fatal(err)
	}
	playlist := "#EXTM3U\n#EXT-X-VERSION:3\n"
	if err := os.WriteFile(filepath.Join(segDir, "index.m3u8"), []byte(playlist), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{Username: "master", Password: "secret", SegmentsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/stream/"+cameraID+"/index.m3u8", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStreamRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret", SegmentsPath: t.TempDir()}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/stream/entrada/index.m3u8", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRecordingsReturnsChunksForDate(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "04", "30")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260430143000.mp4", "20260430143500.mp4"} {
		if err := os.WriteFile(filepath.Join(dateDir, name), []byte("fake"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ServerConfig{Username: "master", Password: "secret", RecordingsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-04-30&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Recordings []map[string]string `json:"recordings"`
		HasMore    bool                `json:"hasMore"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Recordings) != 2 {
		t.Fatalf("expected 2 recordings, got %d", len(resp.Recordings))
	}
	if resp.Recordings[0]["filename"] != "20260430143000.mp4" {
		t.Errorf("unexpected first recording: %v", resp.Recordings[0])
	}
	wantStart := "2026-04-30T14:30:00Z"
	if resp.Recordings[0]["start"] != wantStart {
		t.Errorf("expected start %q, got %q", wantStart, resp.Recordings[0]["start"])
	}
	wantURL := "/recordings/" + cameraID + "/2026/04/30/20260430143000.mp4"
	if resp.Recordings[0]["url"] != wantURL {
		t.Errorf("expected url %q, got %q", wantURL, resp.Recordings[0]["url"])
	}
}

func TestRecordingsSpansMidnightUTCWhenTimezoneOffset(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"

	// 2026-05-02 no fuso America/Sao_Paulo (-3h) começa às 03:00 UTC do dia 02
	// e termina às 02:59:59 UTC do dia 03.
	// Gravações às 01:00 UTC do dia 03 pertencem ao dia local 02.
	day02Dir := filepath.Join(tmpDir, cameraID, "2026", "05", "02")
	day03Dir := filepath.Join(tmpDir, cameraID, "2026", "05", "03")
	for _, dir := range []string{day02Dir, day03Dir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// 10:00 UTC do dia 02 → 07:00 Sao Paulo → pertence ao dia local 02 ✓
	os.WriteFile(filepath.Join(day02Dir, "20260502100000.mp4"), []byte("x"), 0644)
	// 01:00 UTC do dia 03 → 22:00 Sao Paulo do dia 02 → pertence ao dia local 02 ✓
	os.WriteFile(filepath.Join(day03Dir, "20260503010000.mp4"), []byte("x"), 0644)
	// 04:00 UTC do dia 03 → 01:00 Sao Paulo do dia 03 → NÃO pertence ao dia local 02 ✗
	os.WriteFile(filepath.Join(day03Dir, "20260503040000.mp4"), []byte("x"), 0644)

	cfg := config.ServerConfig{Username: "master", Password: "secret", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "America/Sao_Paulo", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-05-02&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Recordings []map[string]string `json:"recordings"`
		HasMore    bool                `json:"hasMore"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Recordings) != 2 {
		t.Fatalf("expected 2 recordings for local day 2026-05-02 in Sao Paulo, got %d: %v", len(resp.Recordings), resp.Recordings)
	}
	filenames := []string{resp.Recordings[0]["filename"], resp.Recordings[1]["filename"]}
	if filenames[0] != "20260502100000.mp4" || filenames[1] != "20260503010000.mp4" {
		t.Errorf("unexpected recordings: %v", filenames)
	}
}

func TestRecordingsRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret", SegmentsPath: t.TempDir()}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/entrada/recordings?date=2026-04-30", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetStatsReturnsStorageInfo(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "01")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "20260501100000.mp4"), []byte("abcde"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "20260501101500.mp4"), []byte("xyz"), 0644); err != nil {
		t.Fatal(err)
	}

	cameras := []config.CameraConfig{{ID: "entrada"}, {ID: "quintal"}}
	cfg := config.ServerConfig{Username: "master", Password: "secret", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		RecordingsBytes int64 `json:"recordings_bytes"`
		RecordingsCount int   `json:"recordings_count"`
		DiskTotalBytes  int64 `json:"disk_total_bytes"`
		DiskFreeBytes   int64 `json:"disk_free_bytes"`
		CameraCount     int   `json:"camera_count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.RecordingsBytes != 8 {
		t.Errorf("expected recordings_bytes=8, got %d", resp.RecordingsBytes)
	}
	if resp.RecordingsCount != 2 {
		t.Errorf("expected recordings_count=2, got %d", resp.RecordingsCount)
	}
	if resp.CameraCount != 2 {
		t.Errorf("expected camera_count=2, got %d", resp.CameraCount)
	}
	if resp.DiskTotalBytes <= 0 {
		t.Errorf("expected disk_total_bytes > 0, got %d", resp.DiskTotalBytes)
	}
	if resp.DiskFreeBytes <= 0 {
		t.Errorf("expected disk_free_bytes > 0, got %d", resp.DiskFreeBytes)
	}
}

func TestGetStatsIncludesMaxSizeWhenConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.ServerConfig{Username: "master", Password: "secret", RecordingsPath: tmpDir}
	storageCfg := config.StorageConfig{MaxSizeGB: 1.0, WarnPercent: 80}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil).
		WithStorageConfig(storageCfg)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		MaxSizeBytes int64 `json:"max_size_bytes"`
		WarnPercent  float64 `json:"warn_percent"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	wantMaxBytes := int64(1.0 * 1024 * 1024 * 1024)
	if resp.MaxSizeBytes != wantMaxBytes {
		t.Errorf("expected max_size_bytes=%d, got %d", wantMaxBytes, resp.MaxSizeBytes)
	}
	if resp.WarnPercent != 80 {
		t.Errorf("expected warn_percent=80, got %f", resp.WarnPercent)
	}
}

func TestGetStatsRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionEventsReturnsEventsForDate(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "03")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	ndjson := `{"time":"2026-05-03T10:00:00Z","score":0.12}` + "\n" +
		`{"time":"2026-05-03T10:00:05Z","score":0.08}` + "\n"
	if err := os.WriteFile(filepath.Join(dateDir, "motion.ndjson"), []byte(ndjson), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{Username: "master", Password: "secret", RecordingsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion?date=2026-05-03", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(resp.Events))
	}
	if resp.Events[0]["time"] != "2026-05-03T10:00:00Z" {
		t.Errorf("unexpected first event time: %v", resp.Events[0]["time"])
	}
}

func TestMotionEventsRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/entrada/motion?date=2026-05-03", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionEventsReturnsEmptyWhenNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.ServerConfig{Username: "master", Password: "secret", RecordingsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: "entrada"}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/entrada/motion?date=2026-05-03", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Events []map[string]any `json:"events"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(resp.Events))
	}
}
