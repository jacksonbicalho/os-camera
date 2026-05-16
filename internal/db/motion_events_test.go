package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func ensureCamera(t *testing.T, database *db.DB, id string) {
	t.Helper()
	if err := db.CreateCamera(database, makeCamera(id), nil); err != nil {
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
