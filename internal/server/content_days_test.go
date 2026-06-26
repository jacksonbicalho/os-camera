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
}
