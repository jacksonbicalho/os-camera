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
