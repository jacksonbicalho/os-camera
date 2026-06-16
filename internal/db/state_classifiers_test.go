package db_test

import (
	"testing"

	"camera/internal/db"
	"camera/internal/stateclass"
)

func makeClassifier(cameraID string) stateclass.Classifier {
	return stateclass.Classifier{
		CameraID:      cameraID,
		Name:          "Portão",
		Model:         "custom-cls",
		Threshold:     0.8,
		TriggerMotion: true,
		CropX:         0.1, CropY: 0.1, CropW: 0.3, CropH: 0.4,
		MinConsecutive: 3,
		Enabled:        true,
		Classes:        []string{"aberto", "fechado"},
	}
}

func seedCamera(t *testing.T, database *db.DB, id string) {
	t.Helper()
	c := makeCamera(id)
	c.ID = id
	if _, err := db.CreateCamera(database, c, nil); err != nil {
		t.Fatal(err)
	}
}

func TestStateClassifierCRUD(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")

	id, err := db.CreateStateClassifier(database, makeClassifier("cam1"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := db.ListStateClassifiers(database, "cam1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if list[0].Name != "Portão" || len(list[0].Classes) != 2 || !list[0].TriggerMotion {
		t.Fatalf("unexpected classifier: %+v", list[0])
	}
	if list[0].Classes[0] != "aberto" || list[0].Classes[1] != "fechado" {
		t.Fatalf("classes order wrong: %v", list[0].Classes)
	}

	got, err := db.GetStateClassifier(database, id)
	if err != nil || got.CropW != 0.3 {
		t.Fatalf("get: %v %+v", err, got)
	}

	got.Name = "Portão lateral"
	got.Classes = []string{"aberto", "fechado", "meio"}
	if err := db.UpdateStateClassifier(database, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, _ := db.GetStateClassifier(database, id)
	if after.Name != "Portão lateral" || len(after.Classes) != 3 {
		t.Fatalf("update not applied: %+v", after)
	}

	if err := db.DeleteStateClassifier(database, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ = db.ListStateClassifiers(database, "cam1")
	if len(list) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(list))
	}
}

func TestStateClassifierCascadeOnCameraDelete(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	if _, err := db.CreateStateClassifier(database, makeClassifier("cam1")); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteCamera(database, "cam1"); err != nil {
		t.Fatalf("delete camera: %v", err)
	}
	list, err := db.ListStateClassifiers(database, "cam1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected classifiers gone after camera delete, got %d", len(list))
	}
}

func TestGetCurrentStateEmpty(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	id, _ := db.CreateStateClassifier(database, makeClassifier("cam1"))
	st, err := db.GetCurrentState(database, id)
	if err != nil {
		t.Fatal(err)
	}
	if st != nil {
		t.Fatalf("expected nil state, got %+v", st)
	}
}
