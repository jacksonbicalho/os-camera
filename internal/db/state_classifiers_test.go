package db_test

import (
	"fmt"
	"reflect"
	"testing"

	"camera/internal/db"
	"camera/internal/stateclass"
)

func TestStateClassifierRecipients(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	u1, err := db.CreateUser(database, "u1", "pw", "viewer", false)
	if err != nil {
		t.Fatal(err)
	}
	u2, err := db.CreateUser(database, "u2", "pw", "viewer", false)
	if err != nil {
		t.Fatal(err)
	}

	c := makeClassifier("cam1")
	c.NotifyEnabled = true
	c.FooterEnabled = true
	c.NotifyUserIDs = []int64{u1, u2}
	c.FooterUserIDs = []int64{u1}
	id, err := db.CreateStateClassifier(database, c)
	if err != nil {
		t.Fatal(err)
	}

	got, err := db.GetStateClassifier(database, id)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NotifyEnabled || !got.FooterEnabled {
		t.Fatalf("flags errados: notify=%v footer=%v", got.NotifyEnabled, got.FooterEnabled)
	}
	if !reflect.DeepEqual(got.NotifyUserIDs, []int64{u1, u2}) {
		t.Fatalf("notify recipients: %v (want %v %v)", got.NotifyUserIDs, u1, u2)
	}
	if !reflect.DeepEqual(got.FooterUserIDs, []int64{u1}) {
		t.Fatalf("footer recipients: %v (want %v)", got.FooterUserIDs, u1)
	}

	// update substitui flags + recipients
	got.NotifyEnabled = false
	got.NotifyUserIDs = []int64{u2}
	got.FooterUserIDs = nil
	if err := db.UpdateStateClassifier(database, got); err != nil {
		t.Fatal(err)
	}
	got2, err := db.GetStateClassifier(database, id)
	if err != nil {
		t.Fatal(err)
	}
	if got2.NotifyEnabled {
		t.Fatal("notify deveria estar off após update")
	}
	if !reflect.DeepEqual(got2.NotifyUserIDs, []int64{u2}) {
		t.Fatalf("notify após update: %v", got2.NotifyUserIDs)
	}
	if len(got2.FooterUserIDs) != 0 {
		t.Fatalf("footer após update deveria estar vazio: %v", got2.FooterUserIDs)
	}

	// delete limpa as chaves em user_permissions (sem FK pro classificador)
	if err := db.DeleteStateClassifier(database, id); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []string{"state_notify", "state_footer"} {
		var n int
		if err := database.QueryRow(
			`SELECT COUNT(*) FROM user_permissions WHERE key = ?`, fmt.Sprintf("%s:%d", ch, id),
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("chaves %s não limpas após delete: %d", ch, n)
		}
	}
}

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

func TestRecordStateTransitionReflectsInCurrent(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	id, _ := db.CreateStateClassifier(database, makeClassifier("cam1"))

	if err := db.RecordStateTransition(database, id, "aberto", 0.91); err != nil {
		t.Fatalf("record: %v", err)
	}
	st, err := db.GetCurrentState(database, id)
	if err != nil || st == nil {
		t.Fatalf("get current: %v %v", err, st)
	}
	if st.State != "aberto" || st.Confidence != 0.91 {
		t.Fatalf("unexpected state: %+v", st)
	}
}
