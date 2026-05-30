package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func mustInsertMotionEvent(t *testing.T, database *db.DB, cameraID string, at time.Time) int64 {
	t.Helper()
	ev := db.MotionEvent{CameraID: cameraID, OccurredAt: at, Score: 0.5}
	if err := db.InsertMotionEvent(database, ev); err != nil {
		t.Fatalf("InsertMotionEvent: %v", err)
	}
	var id int64
	if err := database.QueryRow(`SELECT id FROM motion_events WHERE camera_id=? ORDER BY id DESC LIMIT 1`, cameraID).Scan(&id); err != nil {
		t.Fatalf("get event id: %v", err)
	}
	return id
}

func TestAnnotations_InsertAndList(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	evID := mustInsertMotionEvent(t, database, "cam1", time.Now().UTC())

	ann := db.Annotation{EventID: evID, Label: "gato", BboxX: 10, BboxY: 20, BboxW: 100, BboxH: 80, RotationDeg: 45.5}
	id, err := db.InsertAnnotation(database, ann)
	if err != nil {
		t.Fatalf("InsertAnnotation: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	list, err := db.ListAnnotationsByEvent(database, evID)
	if err != nil {
		t.Fatalf("ListAnnotationsByEvent: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(list))
	}
	got := list[0]
	if got.Label != "gato" || got.BboxX != 10 || got.BboxY != 20 || got.BboxW != 100 || got.BboxH != 80 || got.RotationDeg != 45.5 {
		t.Errorf("unexpected annotation: %+v", got)
	}
}

func TestAnnotations_Delete(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	evID := mustInsertMotionEvent(t, database, "cam1", time.Now().UTC())

	id, _ := db.InsertAnnotation(database, db.Annotation{EventID: evID, Label: "cachorro", BboxX: 1, BboxY: 2, BboxW: 3, BboxH: 4})

	if err := db.DeleteAnnotation(database, id); err != nil {
		t.Fatalf("DeleteAnnotation: %v", err)
	}
	list, _ := db.ListAnnotationsByEvent(database, evID)
	if len(list) != 0 {
		t.Errorf("expected 0 after delete, got %d", len(list))
	}
}

func TestAnnotations_CascadeDeleteWithEvent(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	evID := mustInsertMotionEvent(t, database, "cam1", time.Now().UTC())

	db.InsertAnnotation(database, db.Annotation{EventID: evID, Label: "pessoa", BboxX: 0, BboxY: 0, BboxW: 50, BboxH: 50})

	if _, err := database.Exec(`DELETE FROM motion_events WHERE id=?`, evID); err != nil {
		t.Fatalf("delete event: %v", err)
	}
	list, _ := db.ListAnnotationsByEvent(database, evID)
	if len(list) != 0 {
		t.Errorf("expected 0 after cascade-delete, got %d", len(list))
	}
}

func TestAnnotations_CountAll(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	evID := mustInsertMotionEvent(t, database, "cam1", time.Now().UTC())

	db.InsertAnnotation(database, db.Annotation{EventID: evID, Label: "a", BboxX: 0, BboxY: 0, BboxW: 1, BboxH: 1})
	db.InsertAnnotation(database, db.Annotation{EventID: evID, Label: "b", BboxX: 0, BboxY: 0, BboxW: 1, BboxH: 1})

	n, err := db.CountAnnotations(database)
	if err != nil {
		t.Fatalf("CountAnnotations: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}
