package server_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	"camera/internal/db"
	"camera/internal/motion"
	"camera/internal/server"
	"camera/internal/zones"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoginReturnsTokenForValidCredentials(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	body := `{"username":"master","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
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
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	body := `{"username":"master","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetCamerasReturnsList(t *testing.T) {
	cfg := config.ServerConfig{}
	cameras := []config.CameraConfig{
		{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"},
		{ID: "quintal", RTSPURL: "rtsp://192.168.1.11:554/stream"},
	}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, cameras)

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
	cfg := config.ServerConfig{}
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

	cfg := config.ServerConfig{SegmentsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{SegmentsPath: t.TempDir()}
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

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "America/Sao_Paulo", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{SegmentsPath: t.TempDir()}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/entrada/recordings?date=2026-04-30", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRecordingsHasMotionFromDB(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "24")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20260524150000.mp4", "20260524153000.mp4"} {
		if err := os.WriteFile(filepath.Join(dateDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	// Only the second recording has has_motion=true in the DB.
	db.InsertRecording(database, db.Recording{
		CameraID: cameraID, StartedAt: time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC),
		Path: filepath.Join(tmpDir, cameraID, "2026", "05", "24", "20260524150000.mp4"), HasMotion: false,
	})
	db.InsertRecording(database, db.Recording{
		CameraID: cameraID, StartedAt: time.Date(2026, 5, 24, 15, 30, 0, 0, time.UTC),
		Path: filepath.Join(tmpDir, cameraID, "2026", "05", "24", "20260524153000.mp4"), HasMotion: true,
	})

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)

	token := loginAndGetToken(t, srv, "admin", "pw")
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-05-24&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Recordings []struct {
			Filename  string `json:"filename"`
			HasMotion bool   `json:"has_motion"`
		} `json:"recordings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Recordings) != 2 {
		t.Fatalf("expected 2 recordings, got %d", len(resp.Recordings))
	}
	// default order is desc — 15:30 first
	if resp.Recordings[0].Filename != "20260524153000.mp4" {
		t.Fatalf("unexpected first recording: %v", resp.Recordings[0].Filename)
	}
	if !resp.Recordings[0].HasMotion {
		t.Error("20260524153000.mp4: expected has_motion=true")
	}
	if resp.Recordings[1].HasMotion {
		t.Error("20260524150000.mp4: expected has_motion=false")
	}
}

func TestRecordingsIncludesDBID(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "28")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, "20260528225426.mp4"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	db.InsertRecording(database, db.Recording{
		CameraID:  cameraID,
		StartedAt: time.Date(2026, 5, 28, 22, 54, 26, 0, time.UTC),
		Path:      filepath.Join(tmpDir, cameraID, "2026", "05", "28", "20260528225426.mp4"),
	})

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings?date=2026-05-28&page=1&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Recordings []struct {
			Filename string `json:"filename"`
			ID       int64  `json:"id"`
		} `json:"recordings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Recordings) != 1 {
		t.Fatalf("expected 1 recording, got %d", len(resp.Recordings))
	}
	if resp.Recordings[0].ID == 0 {
		t.Error("expected non-zero DB id in recording listing")
	}
}

func TestGetRecordingByIDReturnsDetails(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "28")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(dateDir, "20260528225426.mp4")
	if err := os.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	db.InsertRecording(database, db.Recording{
		CameraID:  cameraID,
		StartedAt: time.Date(2026, 5, 28, 22, 54, 26, 0, time.UTC),
		Path:      filePath,
	})
	ids, _ := db.IDsByPaths(database, []string{filePath})
	recID := ids[filePath]

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/cameras/%s/recordings/by-id/%d", cameraID, recID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID          int64  `json:"id"`
		Filename    string `json:"filename"`
		Date        string `json:"date"`
		Start       string `json:"start"`
		URL         string `json:"url"`
		IsRecording bool   `json:"is_recording"`
		HasMotion   bool   `json:"has_motion"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ID != recID {
		t.Errorf("id: want %d, got %d", recID, resp.ID)
	}
	if resp.Filename != "20260528225426.mp4" {
		t.Errorf("filename: want 20260528225426.mp4, got %s", resp.Filename)
	}
	if resp.Date != "2026-05-28" {
		t.Errorf("date: want 2026-05-28, got %s", resp.Date)
	}
	wantURL := "/recordings/" + cameraID + "/2026/05/28/20260528225426.mp4"
	if resp.URL != wantURL {
		t.Errorf("url: want %s, got %s", wantURL, resp.URL)
	}
}

func TestGetRecordingByIDReturns404ForUnknownID(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/"+cameraID+"/recordings/by-id/9999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetStatsReturnsStorageInfo(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "entrada"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "01")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	path1 := filepath.Join(dateDir, "20260501100000.mp4")
	path2 := filepath.Join(dateDir, "20260501101500.mp4")
	if err := os.WriteFile(path1, []byte("abcde"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte("xyz"), 0644); err != nil {
		t.Fatal(err)
	}

	cameras := []config.CameraConfig{{ID: "entrada"}, {ID: "quintal"}}
	cfg := config.ServerConfig{RecordingsPath: tmpDir}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	for _, cam := range cameras {
		if _, err := db.CreateCamera(database, cam, nil); err != nil {
			t.Fatal(err)
		}
	}
	t1, _ := time.ParseInLocation("20060102150405", "20260501100000", time.UTC)
	t2, _ := time.ParseInLocation("20060102150405", "20260501101500", time.UTC)
	db.InsertRecording(database, db.Recording{CameraID: "entrada", StartedAt: t1, Path: path1, SizeBytes: 5})
	db.InsertRecording(database, db.Recording{CameraID: "entrada", StartedAt: t2, Path: path2, SizeBytes: 3})

	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)

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
	var paths []string
	for _, name := range []string{"20260501100000.mp4", "20260501100500.mp4"} {
		p := filepath.Join(dateDir, name)
		if err := os.WriteFile(p, []byte("abcdefgh"), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	cameras := []config.CameraConfig{{ID: cameraID}}
	cfg := config.ServerConfig{RecordingsPath: tmpDir}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, cameras[0], nil); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"20260501100000", "20260501100500"} {
		ts, _ := time.ParseInLocation("20060102150405", name, time.UTC)
		db.InsertRecording(database, db.Recording{CameraID: cameraID, StartedAt: ts, Path: paths[i], SizeBytes: 8})
	}

	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)

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
	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	database := openServerTestDB(t)
	for _, u := range []struct{ name, pass, role string }{
		{"master", "secret", "admin"},
	} {
		if _, err := db.CreateUser(database, u.name, u.pass, u.role, false); err != nil {
			t.Fatalf("seed test user %q: %v", u.name, err)
		}
	}
	if err := db.SetConfig(database, "storage.max_size_gb", "1.0"); err != nil {
		t.Fatalf("SetConfig max_size_gb: %v", err)
	}
	if err := db.SetConfig(database, "storage.warn_percent", "80"); err != nil {
		t.Fatalf("SetConfig warn_percent: %v", err)
	}
	srv = srv.WithDB(database)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		MaxSizeBytes int64   `json:"max_size_bytes"`
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
	cfg := config.ServerConfig{}
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
		RecordingsPath: tmpDir,
		SegmentsPath:   tmpDir,
	}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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
	cameras := []config.CameraConfig{{ID: "entrada"}, {ID: "quintal"}}
	cfg := config.ServerConfig{}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	for _, cam := range cameras {
		if _, err := db.CreateCamera(database, cam, nil); err != nil {
			t.Fatal(err)
		}
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	for _, ev := range []db.MotionEvent{
		{CameraID: "entrada", OccurredAt: today.Add(1 * time.Hour), Score: 0.5},
		{CameraID: "entrada", OccurredAt: today.Add(2 * time.Hour), Score: 0.9},
		{CameraID: "entrada", OccurredAt: today.Add(3 * time.Hour), Score: 0.3},
	} {
		if err := db.InsertMotionEvent(database, ev); err != nil {
			t.Fatal(err)
		}
	}

	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)

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
	cameraID := "entrada"
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	t1 := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 3, 10, 0, 5, 0, time.UTC)
	for _, ev := range []db.MotionEvent{
		{CameraID: cameraID, OccurredAt: t1, Score: 0.12, Color: "#ff0000"},
		{CameraID: cameraID, OccurredAt: t2, Score: 0.08},
	} {
		if err := db.InsertMotionEvent(database, ev); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ServerConfig{}
	cameras := []config.CameraConfig{{ID: cameraID}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)

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
	if resp.Events[0]["color"] != "#ff0000" {
		t.Errorf("expected color #ff0000, got %v", resp.Events[0]["color"])
	}
}

func TestMotionEventsRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/entrada/motion?date=2026-05-03", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionEventsReturnsEmptyWhenNoEvents(t *testing.T) {
	database := openServerTestDB(t)
	cameras := []config.CameraConfig{{ID: "entrada"}}
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, cameras[0], nil); err != nil {
		t.Fatal(err)
	}
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)

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
	cameraID := "entrada"
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}

	// 2026-05-02 no fuso America/Sao_Paulo (-3h) começa às 03:00 UTC do dia 02
	// e termina às 02:59:59 UTC do dia 03.
	// 10:00 UTC dia 02 → 07:00 Sao Paulo → pertence ao dia local 02 ✓
	// 01:00 UTC dia 03 → 22:00 Sao Paulo dia 02 → pertence ao dia local 02 ✓
	// 04:00 UTC dia 03 → 01:00 Sao Paulo dia 03 → NÃO pertence ao dia local 02 ✗
	for _, ev := range []db.MotionEvent{
		{CameraID: cameraID, OccurredAt: time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC), Score: 0.5},
		{CameraID: cameraID, OccurredAt: time.Date(2026, 5, 3, 1, 0, 0, 0, time.UTC), Score: 0.7},
		{CameraID: cameraID, OccurredAt: time.Date(2026, 5, 3, 4, 0, 0, 0, time.UTC), Score: 0.9},
	} {
		if err := db.InsertMotionEvent(database, ev); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "America/Sao_Paulo", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil).WithDB(database)

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
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/live", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionLiveReturns404WhenNoFeed(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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

func TestMotionLiveStreamsLabelInSSE(t *testing.T) {
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	srcCh <- motion.Event{Time: time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC), Score: 0.42, Label: "jardim"}
	close(srcCh)
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "jardim") {
		t.Errorf("expected label 'jardim' in SSE body, got %q", body)
	}
}

func TestMotionLiveAcceptsTokenViaQueryParam(t *testing.T) {
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/scores", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionScoresReturns404WhenNoFeed(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{}
	threshold03 := 0.03
	motionFPS := 2
	cameras := []config.CameraConfig{{
		ID: "cam1",
		Motion: &config.MotionConfig{Enabled: true, Threshold: threshold03, FPS: motionFPS},
	}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, cameras)

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
	got, ok := list[0]["motion_threshold"].(float64)
	if !ok {
		t.Fatalf("expected motion_threshold field, got %v", list[0])
	}
	if got != threshold03 {
		t.Errorf("expected threshold 0.03, got %v", got)
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
	serverCfg := config.ServerConfig{Port: 8080}
	storageCfg := config.StorageConfig{Path: "/data"}
	srv := server.NewServer(serverCfg, "America/Sao_Paulo", cameras, discardLogger(), nil).
		WithStorageConfig(storageCfg)
	srv = withTestUsersAndCameras(t, srv, cameras)

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
			Port int `json:"port"`
		} `json:"server"`
		Storage struct {
			Path                 string  `json:"path"`
			WithMotionMinutes    int     `json:"with_motion_minutes"`
			WithoutMotionMinutes int     `json:"without_motion_minutes"`
			MaxSizeGB            float64 `json:"max_size_gb"`
			WarnPercent          float64 `json:"warn_percent"`
		} `json:"storage"`
		Defaults struct {
			ChunkDuration     string `json:"chunk_duration"`
			ReconnectInterval string `json:"reconnect_interval"`
		} `json:"defaults"`
		Cameras []struct {
			ID         string `json:"id"`
			RTSPURL    string `json:"rtsp_url"`
			VideoCodec string `json:"video_codec"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
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
	if resp.Storage.WithMotionMinutes != 10080 {
		t.Errorf("expected with_motion_minutes 10080, got %d", resp.Storage.WithMotionMinutes)
	}
	if resp.Storage.WithoutMotionMinutes != 1440 {
		t.Errorf("expected without_motion_minutes 1440, got %d", resp.Storage.WithoutMotionMinutes)
	}
	if resp.Defaults.ChunkDuration != "5m" {
		t.Errorf("expected chunk_duration 5m, got %q", resp.Defaults.ChunkDuration)
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
	cfg := config.ServerConfig{}
	logCfg := config.LogConfig{Output: "file", Path: "/var/log/camera"}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil).
		WithSystemConfig(true, logCfg)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{}
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
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, cameras)

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

func TestGetSettingsIncludesCameraMotionCaptureResolution(t *testing.T) {
	motion := &config.MotionConfig{
		Enabled:       true,
		Threshold:     0.02,
		FPS:           2,
		CaptureWidth:  480,
		CaptureHeight: 270,
	}
	cameras := []config.CameraConfig{
		{ID: "cam1", RTSPURL: "rtsp://localhost/cam1", Motion: motion},
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, cameras)

	token := loginAndGetToken(t, srv, "admin", "pw")
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var resp struct {
		Cameras []struct {
			Motion *struct {
				CaptureWidth  int `json:"capture_width"`
				CaptureHeight int `json:"capture_height"`
			} `json:"motion"`
		} `json:"cameras"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Cameras) != 1 || resp.Cameras[0].Motion == nil {
		t.Fatalf("expected 1 camera with motion, got %v", resp.Cameras)
	}
	if resp.Cameras[0].Motion.CaptureWidth != 480 {
		t.Errorf("capture_width: got %d, want 480", resp.Cameras[0].Motion.CaptureWidth)
	}
	if resp.Cameras[0].Motion.CaptureHeight != 270 {
		t.Errorf("capture_height: got %d, want 270", resp.Cameras[0].Motion.CaptureHeight)
	}
}

func TestGetAboutReturnsVersionAndUptime(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil).
		WithVersion("v2.0.0").
		WithBuildInfo("abc1234", "2026-05-08T12:00:00Z")
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestClientConfigIncludesVersion(t *testing.T) {
	cfg := config.ServerConfig{}
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
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/daily-peak", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionDailyPeakReturns404WhenNoRawFeed(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
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
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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
	cfg := config.ServerConfig{}
	cameraID := "cam1"
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

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

// --- Zonas de exclusão ---

func newZoneServer(t *testing.T, cameraID string) (http.Handler, string) {
	t.Helper()
	cfg := config.ServerConfig{}
	cameras := []config.CameraConfig{{ID: cameraID, RTSPURL: "rtsp://fake"}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil)
	srv = withTestUsersAndCameras(t, srv, cameras)
	token := loginAndGetToken(t, srv, "u", "p")
	return srv, token
}

func TestGetMotionZonesEmptyByDefault(t *testing.T) {
	srv, token := newZoneServer(t, "cam1")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/zones", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got []zones.Zone
	json.NewDecoder(w.Body).Decode(&got)
	if len(got) != 0 {
		t.Fatalf("expected empty zones, got %v", got)
	}
}

func TestPutMotionZonesThenGet(t *testing.T) {
	srv, token := newZoneServer(t, "cam1")

	body := `[{"x":0.1,"y":0.2,"w":0.3,"h":0.4}]`
	req := httptest.NewRequest(http.MethodPut, "/api/cameras/cam1/motion/zones", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/zones", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var got []zones.Zone
	json.NewDecoder(w2.Body).Decode(&got)
	if len(got) != 1 || got[0].X != 0.1 || got[0].W != 0.3 {
		t.Fatalf("unexpected zones after PUT: %v", got)
	}
}

func TestPutMotionZonesValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{"x fora do range", `[{"x":1.5,"y":0,"w":0.1,"h":0.1}]`, http.StatusBadRequest},
		{"x+w > 1", `[{"x":0.8,"y":0,"w":0.5,"h":0.1}]`, http.StatusBadRequest},
		{"y+h > 1", `[{"x":0,"y":0.8,"w":0.1,"h":0.5}]`, http.StatusBadRequest},
		{"negativo", `[{"x":-0.1,"y":0,"w":0.1,"h":0.1}]`, http.StatusBadRequest},
		{"valido", `[{"x":0.1,"y":0.1,"w":0.5,"h":0.5}]`, http.StatusOK},
		{"array vazio valido", `[]`, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, token := newZoneServer(t, "cam1")
			req := httptest.NewRequest(http.MethodPut, "/api/cameras/cam1/motion/zones", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != tc.code {
				t.Errorf("expected %d, got %d: %s", tc.code, w.Code, w.Body.String())
			}
		})
	}
}

func TestMotionZonesRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/motion/zones", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestMotionZonesUnknownCameraReturns404(t *testing.T) {
	srv, token := newZoneServer(t, "cam1")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/nonexistent/motion/zones", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Snapshot ---

func TestSnapshotReturnsJPEG(t *testing.T) {
	fakeJPEG := []byte{0xFF, 0xD8, 0xFF, 0xD9} // JPEG mínimo válido
	snapFn := func(_ context.Context, _ string) ([]byte, error) { return fakeJPEG, nil }

	cfg := config.ServerConfig{}
	cameras := []config.CameraConfig{{ID: "cam1", RTSPURL: "rtsp://fake"}}
	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).
		WithSnapshotter(snapFn)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %q", ct)
	}
	if got := w.Body.Bytes(); string(got) != string(fakeJPEG) {
		t.Errorf("body mismatch")
	}
}

func TestSnapshotUnknownCameraReturns404(t *testing.T) {
	snapFn := func(_ context.Context, _ string) ([]byte, error) { return nil, nil }
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).
		WithSnapshotter(snapFn)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/nonexistent/snapshot", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSnapshotRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/snapshot", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- DELETE /api/cameras/{id}/recordings/{filename} ---

func TestDeleteRecordingReturns204(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	filename := "20260511100000.mp4"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "11")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, filename), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/"+cameraID+"/recordings/"+filename, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if _, err := os.Stat(filepath.Join(dateDir, filename)); !os.IsNotExist(err) {
		t.Error("expected MP4 to be deleted")
	}
}

func TestDeleteRecordingDeletesFileInPreviousDayDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	// Filename encodes UTC 2026-05-27 00:03:21 (crosses midnight), but the
	// recorder stored it in the previous day's directory (2026/05/26).
	filename := "20260527000321.mp4"
	prevDayDir := filepath.Join(tmpDir, cameraID, "2026", "05", "26")
	if err := os.MkdirAll(prevDayDir, 0755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(prevDayDir, filename)
	if err := os.WriteFile(filePath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/"+cameraID+"/recordings/"+filename, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("expected MP4 in previous-day directory to be deleted")
	}
}

func TestDeleteRecordingReturns204WhenFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/cam1/recordings/20260511100000.mp4", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestDeleteRecordingCleansDBEvenWhenFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	database := openServerTestDB(t)
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}

	// Insert a motion event in the range of the chunk to be deleted.
	chunkStart := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID:   "cam1",
		OccurredAt: chunkStart.Add(30 * time.Second),
		Score:      0.5,
	}); err != nil {
		t.Fatal(err)
	}

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/cam1/recordings/20260511100000.mp4", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	events, err := db.ListMotionEvents(database, "cam1", chunkStart, chunkStart.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("expected motion events to be cleaned up, got %d", len(events))
	}
}

func TestDeleteRecordingReturns404WhenUnknownCamera(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)
	srv = withTestUsers(t, srv)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/unknown/recordings/20260511100000.mp4", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteRecordingRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/cam1/recordings/20260511100000.mp4", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteRecordingCleansMotionEvents(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	filename := "20260511100000.mp4"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "11")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, filename), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	database := openServerTestDB(t)
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	chunkStart := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID:   cameraID,
		OccurredAt: chunkStart.Add(time.Minute),
		Score:      0.05,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/"+cameraID+"/recordings/"+filename, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	count, err := db.CountMotionEvents(database, cameraID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 motion events after recording deletion, got %d", count)
	}
}

func TestDeleteRecordingCleansMotionEventsUsingActualEndedAt(t *testing.T) {
	tmpDir := t.TempDir()
	cameraID := "cam1"
	filename := "20260511100000.mp4"
	dateDir := filepath.Join(tmpDir, cameraID, "2026", "05", "11")
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDir, filename), []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	database := openServerTestDB(t)
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: cameraID}, nil); err != nil {
		t.Fatal(err)
	}
	chunkStart := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	chunkActualEnd := chunkStart.Add(6 * time.Minute) // longer than default 5 min

	// Insert recording row with actual ended_at
	if err := db.InsertRecording(database, db.Recording{
		CameraID:  cameraID,
		StartedAt: chunkStart,
		EndedAt:   chunkActualEnd,
		Path:      filepath.Join(dateDir, filename),
		HasMotion: true,
	}); err != nil {
		t.Fatal(err)
	}
	// Event beyond the default chunk duration — would be missed without actual ended_at
	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID:   cameraID,
		OccurredAt: chunkStart.Add(5*time.Minute + 30*time.Second),
		Score:      0.05,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: cameraID}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodDelete, "/api/cameras/"+cameraID+"/recordings/"+filename, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	count, err := db.CountMotionEvents(database, cameraID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 motion events after recording deletion, got %d", count)
	}
}

func TestConfiguredJWTSecretSurvivesReinit(t *testing.T) {
	cfg := config.ServerConfig{JWTSecret: "fixed-secret-for-testing-32chars!"}

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	srv1 := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv1, "master", "secret")

	srv2 := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil).WithDB(database)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv2.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with same jwt_secret across instances, got %d", w.Code)
	}
}

func TestRandomSecretWhenJWTSecretNotConfigured(t *testing.T) {
	cfg := config.ServerConfig{}

	database1 := openServerTestDB(t)
	if _, err := db.CreateUser(database1, "master", "secret", "admin", false); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	srv1 := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil).WithDB(database1)
	token := loginAndGetToken(t, srv1, "master", "secret")

	database2 := openServerTestDB(t)
	if _, err := db.CreateUser(database2, "master", "secret", "admin", false); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	srv2 := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil).WithDB(database2)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv2.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no jwt_secret (random per boot), got %d", w.Code)
	}
}

func TestGetStatsIncludesSysInfo(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{}, discardLogger(), nil)
	srv = withTestUsers(t, srv)

	token := loginAndGetToken(t, srv, "master", "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		OS            string  `json:"os"`
		PID           int     `json:"pid"`
		CPUPercent    float64 `json:"cpu_percent"`
		MemRSSBytes   int64   `json:"mem_rss_bytes"`
		SysMemTotal   int64   `json:"sys_mem_total_bytes"`
		SysMemFree    int64   `json:"sys_mem_free_bytes"`
		Goroutines    int     `json:"goroutines"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OS == "" {
		t.Error("expected os to be non-empty")
	}
	if resp.PID <= 0 {
		t.Errorf("expected pid > 0, got %d", resp.PID)
	}
	if resp.Goroutines <= 0 {
		t.Errorf("expected goroutines > 0, got %d", resp.Goroutines)
	}
}

func TestGetStatsIncludesCameraHealth(t *testing.T) {
	cameras := []config.CameraConfig{
		{ID: "cam1"},
		{ID: "cam2"},
	}
	cfg := config.ServerConfig{}
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	for _, cam := range cameras {
		if _, err := db.CreateCamera(database, cam, nil); err != nil {
			t.Fatal(err)
		}
	}
	recent := time.Now().UTC().Add(-2 * time.Minute)
	db.InsertRecording(database, db.Recording{CameraID: "cam1", StartedAt: recent, Path: "r1.mp4"})

	srv := server.NewServer(cfg, "UTC", cameras, discardLogger(), nil).WithDB(database)
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
			ID              string     `json:"id"`
			Online          bool       `json:"online"`
			LastRecordingAt *time.Time `json:"last_recording_at"`
			MotionEnabled   bool       `json:"motion_enabled"`
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
		case "cam1":
			if !c.Online {
				t.Error("cam1: expected online=true")
			}
			if c.LastRecordingAt == nil {
				t.Error("cam1: expected last_recording_at to be set")
			}
		case "cam2":
			if c.Online {
				t.Error("cam2: expected online=false")
			}
			if c.LastRecordingAt != nil {
				t.Error("cam2: expected last_recording_at to be nil")
			}
		}
	}
}

// --- PUT /api/events/{id}/frame ---

func TestUpdateEventFrameReplacesFile(t *testing.T) {
	tmpDir := t.TempDir()
	database := openServerTestDB(t)
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}

	// frame_path no banco é só o filename; path completo = RecordingsPath/cameraID/YYYY/MM/DD/filename
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	frameFilename := "20260601100000_motion.jpg"
	frameDir := filepath.Join(tmpDir, "cam1", "2026", "06", "01")
	if err := os.MkdirAll(frameDir, 0755); err != nil {
		t.Fatal(err)
	}
	framePath := filepath.Join(frameDir, frameFilename)
	if err := os.WriteFile(framePath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := db.InsertMotionEvent(database, db.MotionEvent{
		CameraID:   "cam1",
		OccurredAt: base,
		Score:      0.5,
		FramePath:  frameFilename,
	}); err != nil {
		t.Fatal(err)
	}
	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Second))
	eventID := events[0].ID

	cfg := config.ServerConfig{RecordingsPath: tmpDir}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "u", "p")

	newJPEG := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/events/%d/frame", eventID), strings.NewReader(string(newJPEG)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	data, err := os.ReadFile(framePath)
	if err != nil {
		t.Fatalf("expected file at %s: %v", framePath, err)
	}
	if string(data) != string(newJPEG) {
		t.Error("file content not updated")
	}
}

func TestUpdateEventFrameReturns404ForUnknownEvent(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "u", "p", "admin", false); err != nil {
		t.Fatal(err)
	}
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "u", "p")

	req := httptest.NewRequest(http.MethodPut, "/api/events/9999/frame", strings.NewReader("data"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateEventFrameRequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodPut, "/api/events/1/frame", strings.NewReader("data"))
	req.Header.Set("Content-Type", "image/jpeg")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
