package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func setupEventLabelTest(t *testing.T) (srv *server.Server, database *db.DB, token string, eventID int64) {
	t.Helper()
	database = openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	ev := db.MotionEvent{
		CameraID:   "cam1",
		OccurredAt: time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC),
		Score:      0.5,
	}
	if err := db.InsertMotionEvent(database, ev); err != nil {
		t.Fatal(err)
	}
	events, err := db.ListMotionEvents(database, "cam1",
		time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
	)
	if err != nil || len(events) == 0 {
		t.Fatal("expected inserted event")
	}
	eventID = events[0].ID

	cfg := config.ServerConfig{}
	srv = server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token = loginAndGetToken(t, srv, "master", "secret")
	return
}

func TestUpdateEventLabel_SetsLabel(t *testing.T) {
	srv, database, token, eventID := setupEventLabelTest(t)

	body := `{"label":"pessoa"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/events/"+itoa(eventID)+"/label",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	ev, err := db.GetMotionEventByID(database, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Label != "pessoa" {
		t.Errorf("expected label %q, got %q", "pessoa", ev.Label)
	}
}

func TestUpdateEventLabel_ClearsLabel(t *testing.T) {
	srv, database, token, eventID := setupEventLabelTest(t)

	_ = db.UpdateMotionEventLabel(database, eventID, "pessoa")

	body := `{"label":""}`
	req := httptest.NewRequest(http.MethodPatch, "/api/events/"+itoa(eventID)+"/label",
		strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	ev, _ := db.GetMotionEventByID(database, eventID)
	if ev.Label != "" {
		t.Errorf("expected empty label, got %q", ev.Label)
	}
}

func TestUpdateEventLabel_RequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodPatch, "/api/events/1/label", strings.NewReader(`{"label":"x"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUpdateEventLabel_InvalidID(t *testing.T) {
	srv, _, token, _ := setupEventLabelTest(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/events/notanid/label",
		strings.NewReader(`{"label":"x"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPageEvents_ReturnsPagedEvents(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	for i := range 5 {
		_ = db.InsertMotionEvent(database, db.MotionEvent{
			CameraID: "cam1", OccurredAt: base.Add(time.Duration(i) * time.Second), Score: 0.5,
		})
	}

	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/events?page=1&limit=3", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Events []map[string]any `json:"events"`
		Total  int              `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total != 5 {
		t.Errorf("expected total=5, got %d", resp.Total)
	}
	if len(resp.Events) != 3 {
		t.Errorf("expected 3 events on page 1, got %d", len(resp.Events))
	}
}

func TestPageEvents_UnlabeledFilter(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	_ = db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: base, Score: 0.5, Label: "pessoa"})
	_ = db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: base.Add(time.Second), Score: 0.5})

	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/events?unlabeled=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Events []map[string]any `json:"events"`
		Total  int              `json:"total"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Errorf("expected total=1 unlabeled, got %d", resp.Total)
	}
}

func TestPageEvents_LabelSearch(t *testing.T) {
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	_ = db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: base, Score: 0.5, Label: "pessoa"})
	_ = db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: base.Add(time.Second), Score: 0.5, Label: "carro"})
	_ = db.InsertMotionEvent(database, db.MotionEvent{CameraID: "cam1", OccurredAt: base.Add(2 * time.Second), Score: 0.5, Label: "Pessoa com chapéu"})

	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "master", "secret")

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/events?label=pessoa", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct {
		Events []map[string]any `json:"events"`
		Total  int              `json:"total"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("expected total=2 for label search 'pessoa', got %d", resp.Total)
	}
}

func TestPageEvents_RequiresAuth(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/events", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func itoa(n int64) string { return fmt.Sprintf("%d", n) }

func setupBulkTest(t *testing.T) (*server.Server, *db.DB, string, []int64) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "master", "secret", "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1"}, nil); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		_ = db.InsertMotionEvent(database, db.MotionEvent{
			CameraID:   "cam1",
			OccurredAt: base.Add(time.Duration(i) * time.Second),
			Score:      0.5,
		})
	}
	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Hour))
	ids := []int64{}
	for _, ev := range events {
		ids = append(ids, ev.ID)
	}
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", []config.CameraConfig{{ID: "cam1"}}, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "master", "secret")
	return srv, database, token, ids
}

func TestBulkDeleteEvents_DeletesRows(t *testing.T) {
	srv, database, token, ids := setupBulkTest(t)
	body := fmt.Sprintf(`{"ids":[%d,%d]}`, ids[0], ids[1])

	req := httptest.NewRequest(http.MethodDelete, "/api/events/bulk", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct{ Deleted int64 }
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Deleted != 2 {
		t.Errorf("expected deleted=2, got %d", resp.Deleted)
	}
	n, _ := db.CountMotionEvents(database, "cam1")
	if n != 1 {
		t.Errorf("expected 1 remaining, got %d", n)
	}
}

func TestBulkDeleteEvents_EmptyIDsReturnsZero(t *testing.T) {
	srv, _, token, _ := setupBulkTest(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/events/bulk", strings.NewReader(`{"ids":[]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp struct{ Deleted int64 }
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Deleted != 0 {
		t.Errorf("expected 0, got %d", resp.Deleted)
	}
}

func TestBulkDeleteEvents_RequiresAdmin(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/events/bulk", strings.NewReader(`{"ids":[1]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestBulkUpdateEventLabels_AppliesLabel(t *testing.T) {
	srv, database, token, ids := setupBulkTest(t)
	body := fmt.Sprintf(`{"ids":[%d,%d],"label":"cat"}`, ids[0], ids[1])

	req := httptest.NewRequest(http.MethodPatch, "/api/events/bulk/label", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct{ Updated int64 }
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Updated != 2 {
		t.Errorf("expected updated=2, got %d", resp.Updated)
	}
	ev, _ := db.GetMotionEventByID(database, ids[0])
	if ev.Label != "cat" {
		t.Errorf("expected label cat, got %q", ev.Label)
	}
}

func TestBulkUpdateEventLabels_TooManyIDs(t *testing.T) {
	srv, _, token, _ := setupBulkTest(t)
	parts := make([]string, 501)
	for i := range parts {
		parts[i] = "1"
	}
	body := `{"ids":[` + strings.Join(parts, ",") + `],"label":"x"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/events/bulk/label", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestBulkDismissEvents_SetsDismissedFlag(t *testing.T) {
	srv, database, token, ids := setupBulkTest(t)

	body := fmt.Sprintf(`{"ids":[%d,%d]}`, ids[0], ids[1])
	req := httptest.NewRequest(http.MethodPatch, "/api/events/bulk/dismiss", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct{ Dismissed int64 }
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Dismissed != 2 {
		t.Errorf("expected dismissed=2, got %d", resp.Dismissed)
	}

	ev, _ := db.GetMotionEventByID(database, ids[0])
	if !ev.Dismissed {
		t.Error("evento deve estar dismissed após PATCH")
	}
	// evento não dismissado continua no banco
	n, _ := db.CountMotionEvents(database, "cam1")
	if n != 3 {
		t.Errorf("dismiss não deve deletar eventos: esperado 3, got %d", n)
	}
}

func TestBulkDismissEvents_RequiresAdmin(t *testing.T) {
	cfg := config.ServerConfig{}
	srv := server.NewServer(cfg, "UTC", nil, discardLogger(), nil)
	req := httptest.NewRequest(http.MethodPatch, "/api/events/bulk/dismiss", strings.NewReader(`{"ids":[1]}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPageEvents_ExcludesDismissedByDefault(t *testing.T) {
	srv, database, token, ids := setupBulkTest(t)

	// Dismiss o primeiro evento
	body := fmt.Sprintf(`{"ids":[%d]}`, ids[0])
	req := httptest.NewRequest(http.MethodPatch, "/api/events/bulk/dismiss", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("dismiss: expected 200, got %d", w.Code)
	}

	// Lista padrão deve excluir o dismissed
	req2 := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/events", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var resp struct {
		Events []map[string]any `json:"events"`
		Total  int              `json:"total"`
	}
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp.Total != 2 {
		t.Errorf("listagem padrão deve excluir dismissed: esperado total=2, got %d", resp.Total)
	}
	_ = database
}

func TestPageEvents_ShowsDismissedWhenRequested(t *testing.T) {
	srv, _, token, ids := setupBulkTest(t)

	body := fmt.Sprintf(`{"ids":[%d]}`, ids[0])
	req := httptest.NewRequest(http.MethodPatch, "/api/events/bulk/dismiss", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/api/cameras/cam1/events?dismissed=true", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	var resp struct {
		Events []map[string]any `json:"events"`
		Total  int              `json:"total"`
	}
	json.NewDecoder(w2.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Errorf("dismissed=true deve retornar só ignorados: esperado total=1, got %d", resp.Total)
	}
}
