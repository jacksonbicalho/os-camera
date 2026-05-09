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
	"time"

	"camera/internal/config"
	"camera/internal/motion"
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
	// default order is desc — newest first
	if resp.Recordings[0]["filename"] != "20260430143500.mp4" {
		t.Errorf("unexpected first recording: %v", resp.Recordings[0])
	}
	wantStart := "2026-04-30T14:35:00Z"
	if resp.Recordings[0]["start"] != wantStart {
		t.Errorf("expected start %q, got %q", wantStart, resp.Recordings[0]["start"])
	}
	wantURL := "/recordings/" + cameraID + "/2026/04/30/20260430143500.mp4"
	if resp.Recordings[0]["url"] != wantURL {
		t.Errorf("expected url %q, got %q", wantURL, resp.Recordings[0]["url"])
	}
}

func TestRecordingsDefaultOrderIsNewestFirst(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "04", "30")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260430140000.mp4", "20260430143000.mp4", "20260430150000.mp4"} {
		if err := os.WriteFile(filepath.Join(dateDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-04-30&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Recordings []map[string]string `json:"recordings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Recordings) != 3 {
		t.Fatalf("expected 3 recordings, got %d", len(resp.Recordings))
	}
	if resp.Recordings[0]["filename"] != "20260430150000.mp4" {
		t.Errorf("expected newest first, got %q", resp.Recordings[0]["filename"])
	}
}

func TestRecordingsAscOrderIsOldestFirst(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "04", "30")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260430140000.mp4", "20260430143000.mp4", "20260430150000.mp4"} {
		if err := os.WriteFile(filepath.Join(dateDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-04-30&page=1&limit=10&order=asc", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Recordings []map[string]string `json:"recordings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Recordings) != 3 {
		t.Fatalf("expected 3 recordings, got %d", len(resp.Recordings))
	}
	if resp.Recordings[0]["filename"] != "20260430140000.mp4" {
		t.Errorf("expected oldest first, got %q", resp.Recordings[0]["filename"])
	}
}

func TestRecordingsMarksActiveFileAsRecording(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "04", "30")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(dateDir, "20260430140000.mp4")
	activeFile := filepath.Join(dateDir, "20260430143000.mp4")
	for _, f := range []string{oldFile, activeFile} {
		if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-10 * time.Minute)
	os.Chtimes(oldFile, old, old)

	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-04-30&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Recordings []struct {
			Filename    string `json:"filename"`
			IsRecording bool   `json:"is_recording"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(resp.Recordings) != 2 {
		t.Fatalf("expected 2 recordings, got %d", len(resp.Recordings))
	}
	for _, rec := range resp.Recordings {
		if rec.Filename == "20260430140000.mp4" && rec.IsRecording {
			t.Errorf("old file should not be marked as recording")
		}
		if rec.Filename == "20260430143000.mp4" && !rec.IsRecording {
			t.Errorf("active file should be marked as recording")
		}
	}
}

func TestRecordingsOnlyLatestFileIsMarkedAsRecording(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "04", "30")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Both files have recent mtime — simulates the transition moment when a
	// chunk just closed and the new one just opened.
	olderFile := filepath.Join(dateDir, "20260430140000.mp4")
	newerFile := filepath.Join(dateDir, "20260430143000.mp4")
	for _, f := range []string{olderFile, newerFile} {
		if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-04-30&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Recordings []struct {
			Filename    string `json:"filename"`
			IsRecording bool   `json:"is_recording"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	recByName := map[string]bool{}
	for _, r := range resp.Recordings {
		recByName[r.Filename] = r.IsRecording
	}
	if recByName["20260430140000.mp4"] {
		t.Errorf("older file must not be marked as recording when a newer file exists")
	}
	if !recByName["20260430143000.mp4"] {
		t.Errorf("latest file should be marked as recording")
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
	// default order is desc — newest first
	filenames := []string{resp.Recordings[0]["filename"], resp.Recordings[1]["filename"]}
	if filenames[0] != "20260503010000.mp4" || filenames[1] != "20260502100000.mp4" {
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

func TestGetStatsIncludesRecordingsDurationAndForecast(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "01")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260501100000.mp4", "20260501100500.mp4"} {
		if err := os.WriteFile(filepath.Join(dateDir, name), []byte("abcdefgh"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	defaults := config.DefaultsConfig{ChunkDuration: config.Duration(5 * time.Minute)}
	cameras := []config.CameraConfig{{ID: cameraID}}
	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).
		WithDefaults(defaults)

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		RecordingsDurationSeconds int64 `json:"recordings_duration_seconds"`
		ForecastSeconds           int64 `json:"forecast_seconds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	// 2 chunks × 300s = 600s
	if resp.RecordingsDurationSeconds != 600 {
		t.Errorf("expected recordings_duration_seconds=600, got %d", resp.RecordingsDurationSeconds)
	}
	if resp.ForecastSeconds <= 0 {
		t.Errorf("expected forecast_seconds > 0, got %d", resp.ForecastSeconds)
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

func TestGetStatsIncludesConnectedClients(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"
	streamDir := filepath.Join(tmpDir, cameraID)
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(streamDir, "index.m3u8"), []byte("#EXTM3U\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{
		Username:       "master",
		Password:       "secret",
		RecordingsPath: tmpDir,
		SegmentsPath:   tmpDir,
	}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "master", "secret")

	streamReq := httptest.NewRequest(http.MethodGet, "/stream/"+cameraID+"/index.m3u8", nil)
	streamReq.Header.Set("Authorization", "Bearer "+token)
	streamReq.RemoteAddr = "10.10.10.5:12345"
	streamW := httptest.NewRecorder()
	srv.ServeHTTP(streamW, streamReq)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		ConnectedClients int `json:"connected_clients"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ConnectedClients != 1 {
		t.Fatalf("expected connected_clients=1, got %d", resp.ConnectedClients)
	}
}

func TestGetStatsIncludesTopMotionScorePerCamera(t *testing.T) {
	tmpDir := t.TempDir()
	today := time.Now().UTC()
	dateDir := filepath.Join(tmpDir, "entrada", today.Format("2006/01/02"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	ndjson := `{"time":"2026-05-04T10:00:00Z","score":0.5}` + "\n" +
		`{"time":"2026-05-04T10:05:00Z","score":0.9}` + "\n" +
		`{"time":"2026-05-04T10:10:00Z","score":0.3}` + "\n"
	if err := os.WriteFile(filepath.Join(dateDir, "motion.ndjson"), []byte(ndjson), 0644); err != nil {
		t.Fatal(err)
	}

	cameras := []config.CameraConfig{{ID: "entrada"}, {ID: "quintal"}}
	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Cameras []struct {
			ID             string  `json:"id"`
			TopMotionScore float64 `json:"top_motion_score"`
			MinMotionScore float64 `json:"min_motion_score"`
		} `json:"cameras"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Cameras) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(resp.Cameras))
	}
	for _, c := range resp.Cameras {
		switch c.ID {
		case "entrada":
			if c.TopMotionScore != 0.9 {
				t.Errorf("entrada: expected top_motion_score=0.9, got %f", c.TopMotionScore)
			}
			if c.MinMotionScore != 0.3 {
				t.Errorf("entrada: expected min_motion_score=0.3, got %f", c.MinMotionScore)
			}
		case "quintal":
			if c.TopMotionScore != 0.0 {
				t.Errorf("quintal: expected top_motion_score=0.0, got %f", c.TopMotionScore)
			}
			if c.MinMotionScore != 0.0 {
				t.Errorf("quintal: expected min_motion_score=0.0, got %f", c.MinMotionScore)
			}
		}
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

func TestMotionEventsSpansMidnightUTCWhenTimezoneOffset(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"

	// 2026-05-02 no fuso America/Sao_Paulo (-3h) começa às 03:00 UTC do dia 02
	// e termina às 02:59:59 UTC do dia 03.
	day02Dir := filepath.Join(tmpDir, cameraID, "2026", "05", "02")
	day03Dir := filepath.Join(tmpDir, cameraID, "2026", "05", "03")
	for _, dir := range []string{day02Dir, day03Dir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	// 10:00 UTC dia 02 → 07:00 Sao Paulo → pertence ao dia local 02 ✓
	ndjson02 := `{"time":"2026-05-02T10:00:00Z","score":0.5}` + "\n"
	os.WriteFile(filepath.Join(day02Dir, "motion.ndjson"), []byte(ndjson02), 0644)
	// 01:00 UTC dia 03 → 22:00 Sao Paulo dia 02 → pertence ao dia local 02 ✓
	// 04:00 UTC dia 03 → 01:00 Sao Paulo dia 03 → NÃO pertence ao dia local 02 ✗
	ndjson03 := `{"time":"2026-05-03T01:00:00Z","score":0.7}` + "\n" +
		`{"time":"2026-05-03T04:00:00Z","score":0.9}` + "\n"
	os.WriteFile(filepath.Join(day03Dir, "motion.ndjson"), []byte(ndjson03), 0644)

	cfg := config.ServerConfig{Username: "u", Password: "p", RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "America/Sao_Paulo", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion?date=2026-05-02", nil)
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
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("expected 2 events for local day 2026-05-02 in Sao Paulo, got %d: %v", len(resp.Events), resp.Events)
	}
	times := []string{resp.Events[0]["time"].(string), resp.Events[1]["time"].(string)}
	if times[0] != "2026-05-02T10:00:00Z" || times[1] != "2026-05-03T01:00:00Z" {
		t.Errorf("unexpected event times: %v", times)
	}
}

func TestMotionLiveRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/live", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionLiveReturns404WhenNoFeed(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/live", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMotionLiveStreamsSSEEvents(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	srcCh := make(chan motion.Event, 1)
	srv.WithMotionFeed(cameraID, srcCh)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion/live", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(w, req)
	}()

	time.Sleep(10 * time.Millisecond)
	srcCh <- motion.Event{Time: time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC), Score: 0.42}
	close(srcCh)
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "data:") {
		t.Errorf("expected SSE data line in body, got %q", body)
	}
	if !strings.Contains(body, "0.42") {
		t.Errorf("expected score 0.42 in body, got %q", body)
	}
}

func TestMotionLiveAcceptsTokenViaQueryParam(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	srcCh := make(chan motion.Event)
	srv.WithMotionFeed(cameraID, srcCh)
	close(srcCh)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion/live?token="+token, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMotionScoresRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/scores", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionScoresReturns404WhenNoFeed(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/scores", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMotionScoresStreamsSSEEvents(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	srcCh := make(chan motion.Event, 1)
	srv.WithRawFeed(cameraID, srcCh)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion/scores", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(w, req)
	}()

	time.Sleep(10 * time.Millisecond)
	srcCh <- motion.Event{Time: time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC), Score: 0.007}
	close(srcCh)
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "0.007") {
		t.Errorf("expected score 0.007 in body, got %q", body)
	}
}

func TestCamerasIncludesMotionThreshold(t *testing.T) {
	cfg := config.ServerConfig{Username: "u", Password: "p"}
	cameras := []config.CameraConfig{{ID: "cam1"}}
	motionCfg := config.MotionConfig{Enabled: true, Threshold: 0.03, FPS: 2}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).
		WithMotionConfig(motionCfg)

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var list []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(list))
	}
	threshold, ok := list[0]["motion_threshold"].(float64)
	if !ok {
		t.Fatalf("expected motion_threshold field, got %v", list[0])
	}
	if threshold != 0.03 {
		t.Errorf("expected threshold 0.03, got %v", threshold)
	}
}

func TestGetSettingsReturnsFullConfig(t *testing.T) {
	hasAudio := true
	cameras := []config.CameraConfig{
		{
			ID:      "cam1",
			RTSPURL: "rtsp://192.168.1.10:554/stream",
		},
		{
			ID:       "cam2",
			RTSPURL:  "rtsp://user:secret@192.168.1.11:554/stream",
			HasAudio: &hasAudio,
			Width:    1920, Height: 1080,
			VideoCodec: "h264",
		},
	}
	serverCfg := config.ServerConfig{Username: "admin", Password: "pw", Port: 8080, HLSDVRSeconds: 30}
	storageCfg := config.StorageConfig{Path: "/data", RetentionMinutes: 1440, MaxSizeGB: 10, WarnPercent: 80}
	motionCfg := config.MotionConfig{Enabled: true, Threshold: 0.02, FPS: 2, CooldownSeconds: 30}
	defaultsCfg := config.DefaultsConfig{
		ChunkDuration:     config.Duration(5 * time.Minute),
		ReconnectInterval: config.Duration(10 * time.Second),
	}

	srv := server.NewServer(serverCfg, "America/Sao_Paulo", cameras, discardLogger(), nil).
		WithStorageConfig(storageCfg).
		WithMotionConfig(motionCfg).
		WithDefaults(defaultsCfg)

	token := loginAndGetToken(t, srv, "admin", "pw")
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Timezone string `json:"timezone"`
		Server   struct {
			Port          int    `json:"port"`
			Username      string `json:"username"`
			HLSDVRSeconds int    `json:"hls_dvr_seconds"`
		} `json:"server"`
		Storage struct {
			Path             string  `json:"path"`
			RetentionMinutes int     `json:"retention_minutes"`
			MaxSizeGB        float64 `json:"max_size_gb"`
			WarnPercent      float64 `json:"warn_percent"`
		} `json:"storage"`
		Motion struct {
			Enabled         bool    `json:"enabled"`
			Threshold       float64 `json:"threshold"`
			FPS             int     `json:"fps"`
			CooldownSeconds int     `json:"cooldown_seconds"`
		} `json:"motion"`
		Defaults struct {
			ChunkDuration     string `json:"chunk_duration"`
			ReconnectInterval string `json:"reconnect_interval"`
		} `json:"defaults"`
		Cameras []struct {
			ID         string  `json:"id"`
			RTSPURL    string  `json:"rtsp_url"`
			VideoCodec string  `json:"video_codec"`
			Width      int     `json:"width"`
			Height     int     `json:"height"`
		} `json:"cameras"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Timezone != "America/Sao_Paulo" {
		t.Errorf("expected timezone America/Sao_Paulo, got %q", resp.Timezone)
	}
	if resp.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", resp.Server.Port)
	}
	if resp.Server.Username != "admin" {
		t.Errorf("expected username admin, got %q", resp.Server.Username)
	}
	if resp.Server.HLSDVRSeconds != 30 {
		t.Errorf("expected hls_dvr_seconds 30, got %d", resp.Server.HLSDVRSeconds)
	}
	if resp.Storage.RetentionMinutes != 1440 {
		t.Errorf("expected retention_minutes 1440, got %d", resp.Storage.RetentionMinutes)
	}
	if resp.Motion.Threshold != 0.02 {
		t.Errorf("expected threshold 0.02, got %f", resp.Motion.Threshold)
	}
	if resp.Defaults.ChunkDuration != "5m0s" {
		t.Errorf("expected chunk_duration 5m0s, got %q", resp.Defaults.ChunkDuration)
	}
	if resp.Defaults.ReconnectInterval != "10s" {
		t.Errorf("expected reconnect_interval 10s, got %q", resp.Defaults.ReconnectInterval)
	}
	if len(resp.Cameras) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(resp.Cameras))
	}
	if resp.Cameras[1].VideoCodec != "h264" {
		t.Errorf("expected video_codec h264, got %q", resp.Cameras[1].VideoCodec)
	}
}

func TestGetSettingsIncludesDebugAndLog(t *testing.T) {
	cfg := config.ServerConfig{Username: "u", Password: "p"}
	logCfg := config.LogConfig{Output: "file", Path: "/var/log/camera"}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil).
		WithSystemConfig(true, logCfg)

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Debug bool `json:"debug"`
		Log   struct {
			Output string `json:"output"`
			Path   string `json:"path"`
		} `json:"log"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Debug {
		t.Error("expected debug=true")
	}
	if resp.Log.Output != "file" {
		t.Errorf("expected log.output=file, got %q", resp.Log.Output)
	}
	if resp.Log.Path != "/var/log/camera" {
		t.Errorf("expected log.path=/var/log/camera, got %q", resp.Log.Path)
	}
}

func TestGetSettingsRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGetSettingsMasksRTSPCredentials(t *testing.T) {
	cameras := []config.CameraConfig{
		{ID: "cam1", RTSPURL: "rtsp://user:s3cr3t@192.168.1.10:554/live"},
		{ID: "cam2", RTSPURL: "rtsp://192.168.1.11:554/stream"},
	}
	cfg := config.ServerConfig{Username: "u", Password: "p"}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Cameras []struct {
			RTSPURL string `json:"rtsp_url"`
		} `json:"cameras"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Cameras) != 2 {
		t.Fatalf("expected 2 cameras, got %d", len(resp.Cameras))
	}
	if strings.Contains(resp.Cameras[0].RTSPURL, "s3cr3t") {
		t.Errorf("password must not appear in rtsp_url, got %q", resp.Cameras[0].RTSPURL)
	}
	if !strings.Contains(resp.Cameras[0].RTSPURL, "user") {
		t.Errorf("username should remain visible, got %q", resp.Cameras[0].RTSPURL)
	}
	if resp.Cameras[1].RTSPURL != "rtsp://192.168.1.11:554/stream" {
		t.Errorf("url without credentials should remain unchanged, got %q", resp.Cameras[1].RTSPURL)
	}
}

func TestGetAboutReturnsVersionAndUptime(t *testing.T) {
	cfg := config.ServerConfig{Username: "u", Password: "p"}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil).
		WithVersion("v2.0.0").
		WithBuildInfo("abc1234", "2026-05-08T12:00:00Z")

	token := loginAndGetToken(t, srv, "u", "p")
	req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Version       string  `json:"version"`
		Commit        string  `json:"commit"`
		BuiltAt       string  `json:"built_at"`
		UptimeSeconds float64 `json:"uptime_seconds"`
		GoVersion     string  `json:"go_version"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Version != "v2.0.0" {
		t.Errorf("expected version v2.0.0, got %q", resp.Version)
	}
	if resp.Commit != "abc1234" {
		t.Errorf("expected commit abc1234, got %q", resp.Commit)
	}
	if resp.BuiltAt != "2026-05-08T12:00:00Z" {
		t.Errorf("expected built_at 2026-05-08T12:00:00Z, got %q", resp.BuiltAt)
	}
	if resp.UptimeSeconds < 0 {
		t.Errorf("expected uptime_seconds >= 0, got %f", resp.UptimeSeconds)
	}
	if resp.GoVersion == "" {
		t.Errorf("expected non-empty go_version")
	}
}

func TestGetAboutRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestClientConfigIncludesVersion(t *testing.T) {
	cfg := config.ServerConfig{Username: "u", Password: "p"}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil).
		WithVersion("v1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["version"] != "v1.2.3" {
		t.Errorf("expected version v1.2.3, got %q", resp["version"])
	}
}

func TestMotionDailyPeakRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/daily-peak", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionDailyPeakReturns404WhenNoRawFeed(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/daily-peak", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestMotionDailyPeakReturnsZeroWhenNoEventsYet(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	rawCh := make(chan motion.Event)
	srv.WithRawFeed(cameraID, rawCh)
	close(rawCh)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion/daily-peak", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		CameraID     string  `json:"camera_id"`
		PeakRawScore float64 `json:"peak_raw_score"`
		Date         string  `json:"date"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.PeakRawScore != 0 {
		t.Errorf("expected peak_raw_score=0, got %f", resp.PeakRawScore)
	}
	if resp.CameraID != cameraID {
		t.Errorf("expected camera_id=%q, got %q", cameraID, resp.CameraID)
	}
}

func TestMotionDailyPeakReturnsPeakScore(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	rawCh := make(chan motion.Event, 4)
	srv.WithRawFeed(cameraID, rawCh)

	today := time.Now().UTC()
	rawCh <- motion.Event{Time: today, Score: 30.0}
	rawCh <- motion.Event{Time: today, Score: 142.7}
	rawCh <- motion.Event{Time: today, Score: 85.5}
	close(rawCh)
	time.Sleep(20 * time.Millisecond) // aguarda goroutine processar

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion/daily-peak", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		PeakRawScore float64 `json:"peak_raw_score"`
		Date         string  `json:"date"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.PeakRawScore != 142.7 {
		t.Errorf("expected peak_raw_score=142.7, got %f", resp.PeakRawScore)
	}
	wantDate := today.Format("2006-01-02")
	if resp.Date != wantDate {
		t.Errorf("expected date=%q, got %q", wantDate, resp.Date)
	}
}

func TestMotionDailyPeakResetsOnNewDay(t *testing.T) {
	cfg := config.ServerConfig{Username: "master", Password: "secret"}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)

	rawCh := make(chan motion.Event, 2)
	srv.WithRawFeed(cameraID, rawCh)

	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	today := time.Now().UTC()
	rawCh <- motion.Event{Time: yesterday, Score: 999.0}
	rawCh <- motion.Event{Time: today, Score: 55.0}
	close(rawCh)
	time.Sleep(20 * time.Millisecond)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/motion/daily-peak", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		PeakRawScore float64 `json:"peak_raw_score"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.PeakRawScore != 55.0 {
		t.Errorf("expected peak_raw_score=55.0 (ontem descartado), got %f", resp.PeakRawScore)
	}
}
