package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"camera/internal/analysis"
	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func setupFinetuneServer(t *testing.T, yoloHandler http.HandlerFunc) (http.Handler, string) {
	t.Helper()
	yoloSrv := httptest.NewServer(yoloHandler)
	t.Cleanup(yoloSrv.Close)

	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{
		Enabled:    true,
		ServiceURL: yoloSrv.URL,
		Model:      "yolov8n",
	}); err != nil {
		t.Fatalf("set analysis config: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, token
}

func TestFinetuneStatus_JobNotFound_ReturnsErrorStatus(t *testing.T) {
	// YOLO service returns 404 — job lost after OOM restart
	yoloHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv, token := setupFinetuneServer(t, yoloHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/settings/analysis/finetune/status/lost-job-id", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var status analysis.FinetuneStatus
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if status.Status != "error" {
		t.Errorf("expected status=error, got %q", status.Status)
	}
	if status.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestFinetuneStatus_ServiceDown_Returns502(t *testing.T) {
	// YOLO service is completely unreachable (connection refused)
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.UpdateVideoAnalysisConfig(database, db.VideoAnalysisConfig{
		Enabled:    true,
		ServiceURL: "http://127.0.0.1:19999", // nothing listening here
		Model:      "yolov8n",
	}); err != nil {
		t.Fatalf("set analysis config: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")

	req := httptest.NewRequest(http.MethodGet, "/api/settings/analysis/finetune/status/some-job", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when service unreachable, got %d", rr.Code)
	}
}
