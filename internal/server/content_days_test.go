package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func TestContentDays_Handler(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	cam := config.CameraConfig{ID: "cam1", Name: "Cam", RTSPURL: "rtsp://x"}
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("create camera: %v", err)
	}
	at := func(s string) time.Time { ts, _ := time.Parse(time.RFC3339, s); return ts }
	if err := db.InsertRecording(database, db.Recording{CameraID: "cam1", StartedAt: at("2026-06-20T12:00:00Z"), Path: "r1.mp4"}); err != nil {
		t.Fatalf("rec: %v", err)
	}
	if err := db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: at("2026-06-22T12:00:00Z")}); err != nil {
		t.Fatalf("event: %v", err)
	}

	srv := server.NewServer(config.ServerConfig{}, "UTC", []config.CameraConfig{cam}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/content-days", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Days []string `json:"days"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []string{"2026-06-20", "2026-06-22"}
	if len(resp.Days) != len(want) || resp.Days[0] != want[0] || resp.Days[1] != want[1] {
		t.Fatalf("expected %v, got %v", want, resp.Days)
	}

	// kind=recordings → só o dia da gravação.
	req2 := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/content-days?kind=recordings", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	var r2 struct {
		Days []string `json:"days"`
	}
	json.NewDecoder(w2.Body).Decode(&r2)
	if len(r2.Days) != 1 || r2.Days[0] != "2026-06-20" {
		t.Fatalf("kind=recordings: expected [2026-06-20], got %v", r2.Days)
	}
}

func TestGlobalContentDays_AggregatesAndRespectsAccess(t *testing.T) {
	database := openServerTestDB(t)
	viewerID, err := db.CreateUser(database, "vw", "pw", "viewer", false)
	if err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	for _, cid := range []string{"cam1", "cam2"} {
		if _, err := db.CreateCamera(database, config.CameraConfig{ID: cid, Name: cid, RTSPURL: "rtsp://x"}, nil); err != nil {
			t.Fatalf("create camera %s: %v", cid, err)
		}
	}
	// viewer só enxerga cam1.
	if err := db.SetUserCameras(database, viewerID, []string{"cam1"}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	at := func(s string) time.Time { ts, _ := time.Parse(time.RFC3339, s); return ts }
	_ = db.InsertRecording(database, db.Recording{CameraID: "cam1", StartedAt: at("2026-06-20T12:00:00Z"), Path: "a.mp4"})
	_ = db.InsertRecording(database, db.Recording{CameraID: "cam2", StartedAt: at("2026-06-25T12:00:00Z"), Path: "b.mp4"})

	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "vw", "pw")

	// viewer: só cam1 → não vê o dia de cam2.
	req := httptest.NewRequest(http.MethodGet, "/api/content-days?kind=recordings", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Days []string `json:"days"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Days) != 1 || resp.Days[0] != "2026-06-20" {
		t.Fatalf("viewer should see only cam1 day [2026-06-20], got %v", resp.Days)
	}
}
