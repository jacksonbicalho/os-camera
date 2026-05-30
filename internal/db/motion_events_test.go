package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func ensureCamera(t *testing.T, database *db.DB, id string) {
	t.Helper()
	c := makeCamera(id)
	c.ID = id
	if _, err := db.CreateCamera(database, c, nil); err != nil {
		t.Fatalf("CreateCamera(%s): %v", id, err)
	}
}

func insertTestEvent(t *testing.T, database *db.DB, cameraID string, occurredAt time.Time, score float64, color string) {
	t.Helper()
	ev := db.MotionEvent{
		CameraID:   cameraID,
		OccurredAt: occurredAt,
		Score:      score,
		Color:      color,
	}
	if err := db.InsertMotionEvent(database, ev); err != nil {
		t.Fatalf("InsertMotionEvent: %v", err)
	}
}

func TestListMotionEvents_ReturnsEventsInRange(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "#ff0000")
	insertTestEvent(t, database, "cam1", base.Add(5*time.Second), 0.8, "#00ff00")
	insertTestEvent(t, database, "cam1", base.Add(24*time.Hour), 0.3, "") // dia seguinte — fora do range

	start := base
	end := base.Add(24 * time.Hour)
	events, err := db.ListMotionEvents(database, "cam1", start, end)
	if err != nil {
		t.Fatalf("ListMotionEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Score != 0.5 {
		t.Errorf("expected score 0.5, got %f", events[0].Score)
	}
	if events[0].Color != "#ff0000" {
		t.Errorf("expected color #ff0000, got %q", events[0].Color)
	}
	if events[1].Score != 0.8 {
		t.Errorf("expected score 0.8, got %f", events[1].Score)
	}
}

func TestListMotionEvents_ExcludesOtherCameras(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	ensureCamera(t, database, "cam2")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam2", base, 0.9, "")

	events, err := db.ListMotionEvents(database, "cam1", base, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListMotionEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event for cam1, got %d", len(events))
	}
}

func TestListMotionEvents_ReturnsEmptyWhenNone(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	events, err := db.ListMotionEvents(database, "cam1", base, base.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListMotionEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestMinMaxScoreForDay_ReturnsCorrectValues(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(5*time.Minute), 0.9, "")
	insertTestEvent(t, database, "cam1", base.Add(10*time.Minute), 0.3, "")

	start := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	mn, mx, err := db.MinMaxScoreForDay(database, "cam1", start, end)
	if err != nil {
		t.Fatalf("MinMaxScoreForDay: %v", err)
	}
	if mn != 0.3 {
		t.Errorf("expected min=0.3, got %f", mn)
	}
	if mx != 0.9 {
		t.Errorf("expected max=0.9, got %f", mx)
	}
}

func TestMinMaxScoreForDay_ReturnsZerosWhenEmpty(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	start := time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	mn, mx, err := db.MinMaxScoreForDay(database, "cam1", start, end)
	if err != nil {
		t.Fatalf("MinMaxScoreForDay: %v", err)
	}
	if mn != 0 || mx != 0 {
		t.Errorf("expected 0,0 for empty, got %f,%f", mn, mx)
	}
}

func TestUpdateMotionEventLabel_SetsLabel(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Second))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	id := events[0].ID

	if err := db.UpdateMotionEventLabel(database, id, "pessoa"); err != nil {
		t.Fatalf("UpdateMotionEventLabel: %v", err)
	}

	ev, err := db.GetMotionEventByID(database, id)
	if err != nil {
		t.Fatalf("GetMotionEventByID: %v", err)
	}
	if ev.Label != "pessoa" {
		t.Errorf("expected label %q, got %q", "pessoa", ev.Label)
	}
}

func TestUpdateMotionEventLabel_ClearsLabel(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	ev := db.MotionEvent{CameraID: "cam1", OccurredAt: base, Score: 0.5, Label: "pessoa"}
	if err := db.InsertMotionEvent(database, ev); err != nil {
		t.Fatalf("InsertMotionEvent: %v", err)
	}

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Second))
	id := events[0].ID

	if err := db.UpdateMotionEventLabel(database, id, ""); err != nil {
		t.Fatalf("UpdateMotionEventLabel: %v", err)
	}

	got, _ := db.GetMotionEventByID(database, id)
	if got.Label != "" {
		t.Errorf("expected empty label, got %q", got.Label)
	}
}

func TestPageMotionEvents_ReturnsPagedResults(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	for i := range 5 {
		insertTestEvent(t, database, "cam1", base.Add(time.Duration(i)*time.Second), 0.5, "")
	}

	events, total, err := db.PageMotionEvents(database, "cam1", 0, 3, false, "")
	if err != nil {
		t.Fatalf("PageMotionEvents: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events on page, got %d", len(events))
	}

	events2, _, _ := db.PageMotionEvents(database, "cam1", 3, 3, false, "")
	if len(events2) != 2 {
		t.Errorf("expected 2 events on second page, got %d", len(events2))
	}
}

func TestPageMotionEvents_UnlabeledFilter(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(time.Second), 0.5, "")

	// label the first event
	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Minute))
	_ = db.UpdateMotionEventLabel(database, events[0].ID, "pessoa")

	unlabeled, total, err := db.PageMotionEvents(database, "cam1", 0, 10, true, "")
	if err != nil {
		t.Fatalf("PageMotionEvents unlabeled: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 unlabeled, got %d", total)
	}
	if len(unlabeled) != 1 {
		t.Errorf("expected 1 unlabeled event, got %d", len(unlabeled))
	}
	if unlabeled[0].Label != "" {
		t.Errorf("expected empty label, got %q", unlabeled[0].Label)
	}
}

func TestPageMotionEvents_OrderedNewestFirst(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.1, "")
	insertTestEvent(t, database, "cam1", base.Add(time.Second), 0.2, "")

	events, _, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].OccurredAt.Before(events[1].OccurredAt) {
		t.Error("expected newest first ordering")
	}
}

func TestPageMotionEvents_LabelSearch(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(time.Second), 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(2*time.Second), 0.5, "")

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Minute))
	_ = db.UpdateMotionEventLabel(database, events[2].ID, "pessoa")
	_ = db.UpdateMotionEventLabel(database, events[1].ID, "carro")
	_ = db.UpdateMotionEventLabel(database, events[0].ID, "Pessoa com chapéu")

	// search "pessoa" should match 2 events (case-insensitive)
	results, total, err := db.PageMotionEvents(database, "cam1", 0, 10, false, "pessoa")
	if err != nil {
		t.Fatalf("PageMotionEvents label search: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2 for label search 'pessoa', got %d", total)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// empty search returns all
	all, totalAll, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "")
	if totalAll != 3 {
		t.Errorf("expected total=3 for empty search, got %d", totalAll)
	}
	_ = all
}

func TestInsertMotionEvent_PersistsColor(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "#3b82f6")

	events, err := db.ListMotionEvents(database, "cam1", base, base.Add(time.Second))
	if err != nil {
		t.Fatalf("ListMotionEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Color != "#3b82f6" {
		t.Errorf("expected color #3b82f6, got %q", events[0].Color)
	}
}
