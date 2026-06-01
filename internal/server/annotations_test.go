package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/server"
)

func setupAnnotationsServer(t *testing.T) (http.Handler, *db.DB, string) {
	t.Helper()
	database := openServerTestDB(t)
	if _, err := db.CreateUser(database, "admin", "pw", "admin", false); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	srv := server.NewServer(config.ServerConfig{}, "UTC", nil, discardLogger(), nil).WithDB(database)
	token := loginAndGetToken(t, srv, "admin", "pw")
	return srv, database, token
}

func insertAnnotationTestCamera(t *testing.T, database *db.DB) {
	t.Helper()
	if _, err := db.CreateCamera(database, config.CameraConfig{ID: "cam1", Name: "cam1", RTSPURL: "rtsp://fake"}, nil); err != nil {
		t.Fatalf("create camera: %v", err)
	}
}

func insertAnnotationTestEvent(t *testing.T, database *db.DB) int64 {
	t.Helper()
	ev := db.MotionEvent{CameraID: "cam1", OccurredAt: time.Now().UTC(), Score: 0.5}
	if err := db.InsertMotionEvent(database, ev); err != nil {
		t.Fatalf("InsertMotionEvent: %v", err)
	}
	var id int64
	if err := database.QueryRow(`SELECT id FROM motion_events WHERE camera_id='cam1' ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("get event id: %v", err)
	}
	return id
}

func TestAnnotations_CreateAndList(t *testing.T) {
	srv, database, token := setupAnnotationsServer(t)
	insertAnnotationTestCamera(t, database)
	evID := insertAnnotationTestEvent(t, database)

	body, _ := json.Marshal(map[string]any{
		"label":  "gato",
		"bbox_x": 10.0, "bbox_y": 20.0, "bbox_w": 100.0, "bbox_h": 80.0,
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/events/%d/annotations", evID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST annotations: got %d, body: %s", rr.Code, rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/events/%d/annotations", evID), nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("GET annotations: got %d", rr2.Code)
	}
	var list []db.Annotation
	json.NewDecoder(rr2.Body).Decode(&list)
	if len(list) != 1 || list[0].Label != "gato" {
		t.Errorf("expected 1 annotation 'gato', got %v", list)
	}
}

func TestAnnotations_Delete(t *testing.T) {
	srv, database, token := setupAnnotationsServer(t)
	insertAnnotationTestCamera(t, database)
	evID := insertAnnotationTestEvent(t, database)

	annID, _ := db.InsertAnnotation(database, db.Annotation{EventID: evID, Label: "cão", BboxX: 1, BboxY: 2, BboxW: 3, BboxH: 4})

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/annotations/%d", annID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE annotation: got %d", rr.Code)
	}

	list, _ := db.ListAnnotationsByEvent(database, evID)
	if len(list) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(list))
	}
}

func TestAnnotations_Update(t *testing.T) {
	srv, database, token := setupAnnotationsServer(t)
	insertAnnotationTestCamera(t, database)
	evID := insertAnnotationTestEvent(t, database)

	annID, err := db.InsertAnnotation(database, db.Annotation{
		EventID: evID, Label: "original", BboxX: 10, BboxY: 20, BboxW: 100, BboxH: 80,
	})
	if err != nil {
		t.Fatalf("InsertAnnotation: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"label":  "atualizado",
		"bbox_x": 50.0, "bbox_y": 60.0, "bbox_w": 200.0, "bbox_h": 150.0, "rotation_deg": 45.0,
	})
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/annotations/%d", annID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("PATCH annotation: got %d, body: %s", rr.Code, rr.Body.String())
	}

	list, _ := db.ListAnnotationsByEvent(database, evID)
	if len(list) != 1 {
		t.Fatalf("expected 1 annotation after update, got %d", len(list))
	}
	a := list[0]
	if a.Label != "atualizado" || a.BboxX != 50 || a.BboxY != 60 || a.BboxW != 200 || a.BboxH != 150 || a.RotationDeg != 45 {
		t.Errorf("annotation not updated correctly: %+v", a)
	}
}

func TestAnnotations_RequiresAuth(t *testing.T) {
	srv, database, _ := setupAnnotationsServer(t)
	insertAnnotationTestCamera(t, database)
	evID := insertAnnotationTestEvent(t, database)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/events/%d/annotations", evID), nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
