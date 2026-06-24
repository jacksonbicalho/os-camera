package db_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"camera/internal/db"
	"camera/internal/stateclass"
)

func TestListStateHistory(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	id, _ := db.CreateStateClassifier(database, makeClassifier("cam1"))

	// duas transições; a 2ª é a mais recente (deve vir primeiro)
	if err := db.RecordStateTransition(database, id, "vazio", 0.8, "/recordings/state_history/1/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordStateTransition(database, id, "cheio", 0.9, "/recordings/state_history/1/b.jpg"); err != nil {
		t.Fatal(err)
	}
	// registro SEM imagem (legado/falha) não deve ser listado
	if err := db.RecordStateTransition(database, id, "semfoto", 0.7, ""); err != nil {
		t.Fatal(err)
	}

	// gravação cobrindo "agora" → recording_available verdadeiro para ambas
	now := time.Now().UTC()
	if err := db.InsertRecording(database, db.Recording{
		CameraID: "cam1", StartedAt: now.Add(-1 * time.Hour), EndedAt: now.Add(1 * time.Hour), Path: "cam1/rec.mp4",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := db.ListStateHistory(database, id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("esperava 2 entradas, got %d", len(got))
	}
	if got[0].State != "cheio" || got[0].FramePath != "/recordings/state_history/1/b.jpg" {
		t.Fatalf("mais recente primeiro esperado: %+v", got[0])
	}
	if !got[0].RecordingAvailable || !got[1].RecordingAvailable {
		t.Fatalf("gravação cobre as transições — recording_available deveria ser true: %+v", got)
	}
}

func TestListStateHistoryRecordingExpired(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	id, _ := db.CreateStateClassifier(database, makeClassifier("cam1"))

	if err := db.RecordStateTransition(database, id, "vazio", 0.8, "/recordings/state_history/1/a.jpg"); err != nil {
		t.Fatal(err)
	}
	// gravação antiga, não cobre "agora" → recording_available falso
	old := time.Now().UTC().Add(-48 * time.Hour)
	if err := db.InsertRecording(database, db.Recording{
		CameraID: "cam1", StartedAt: old, EndedAt: old.Add(1 * time.Hour), Path: "cam1/old.mp4",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := db.ListStateHistory(database, id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RecordingAvailable {
		t.Fatalf("gravação expirada — recording_available deveria ser false: %+v", got)
	}
}

func TestFooterClassifiersForUser(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	u1, _ := db.CreateUser(database, "u1", "pw", "viewer", false)
	u2, _ := db.CreateUser(database, "u2", "pw", "viewer", false)

	// c1: footer ligado + u1 destinatário → u1 vê
	c1 := makeClassifier("cam1")
	c1.Name = "Corredor"
	c1.FooterEnabled = true
	c1.FooterUserIDs = []int64{u1}
	id1, err := db.CreateStateClassifier(database, c1)
	if err != nil {
		t.Fatal(err)
	}
	// c2: footer ligado mas só u2 → u1 não vê
	c2 := makeClassifier("cam1")
	c2.FooterEnabled = true
	c2.FooterUserIDs = []int64{u2}
	if _, err := db.CreateStateClassifier(database, c2); err != nil {
		t.Fatal(err)
	}
	// c3: u1 destinatário mas footer desligado → u1 não vê
	c3 := makeClassifier("cam1")
	c3.FooterEnabled = false
	c3.FooterUserIDs = []int64{u1}
	if _, err := db.CreateStateClassifier(database, c3); err != nil {
		t.Fatal(err)
	}

	got, err := db.FooterClassifiersForUser(database, u1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != id1 || got[0].Name != "Corredor" {
		t.Fatalf("u1 deveria ver só o Corredor: %+v", got)
	}
}

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

	if err := db.RecordStateTransition(database, id, "aberto", 0.91, ""); err != nil {
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

func insertTransition(t *testing.T, database *db.DB, classifierID int64, state string, conf float64, frame, changedAt string) {
	t.Helper()
	if _, err := database.Exec(
		`INSERT INTO camera_state_history (classifier_id, state, confidence, frame_path, changed_at) VALUES (?, ?, ?, ?, ?)`,
		classifierID, state, conf, frame, changedAt,
	); err != nil {
		t.Fatal(err)
	}
}

func TestListCameraStateTransitions(t *testing.T) {
	database := openTestDB(t)
	seedCamera(t, database, "cam1")
	seedCamera(t, database, "cam2")
	c1, _ := db.CreateStateClassifier(database, makeClassifier("cam1"))
	c2cfg := makeClassifier("cam1")
	c2cfg.Name = "Garagem"
	c2, _ := db.CreateStateClassifier(database, c2cfg)
	other, _ := db.CreateStateClassifier(database, makeClassifier("cam2"))

	// dentro do dia 2026-05-03, dois classificadores da mesma câmera
	insertTransition(t, database, c1, "vazio", 0.8, "/recordings/state_history/1/a.jpg", "2026-05-03 10:00:00")
	insertTransition(t, database, c2, "aberto", 0.9, "/recordings/state_history/2/b.jpg", "2026-05-03 11:00:00")
	// sem frame_path → não entra
	insertTransition(t, database, c1, "cheio", 0.7, "", "2026-05-03 12:00:00")
	// fora do intervalo → não entra
	insertTransition(t, database, c1, "cheio", 0.6, "/recordings/state_history/1/c.jpg", "2026-05-04 09:00:00")
	// outra câmera → não entra
	insertTransition(t, database, other, "x", 0.5, "/recordings/state_history/3/d.jpg", "2026-05-03 13:00:00")

	start := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	got, err := db.ListCameraStateTransitions(database, "cam1", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("esperava 2 transições, got %d: %+v", len(got), got)
	}
	byState := map[string]db.CameraStateTransition{}
	for _, e := range got {
		byState[e.State] = e
	}
	if byState["vazio"].ClassifierID != c1 || byState["vazio"].ClassifierName != "Portão" {
		t.Fatalf("classificador errado p/ vazio: %+v", byState["vazio"])
	}
	if byState["aberto"].ClassifierName != "Garagem" {
		t.Fatalf("classifier_name esperado Garagem: %+v", byState["aberto"])
	}
	if byState["aberto"].FramePath != "/recordings/state_history/2/b.jpg" || byState["aberto"].Confidence != 0.9 {
		t.Fatalf("campos errados p/ aberto: %+v", byState["aberto"])
	}
}
