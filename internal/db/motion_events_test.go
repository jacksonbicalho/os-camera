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

	events, total, err := db.PageMotionEvents(database, "cam1", 0, 3, false, "", false)
	if err != nil {
		t.Fatalf("PageMotionEvents: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events on page, got %d", len(events))
	}

	events2, _, _ := db.PageMotionEvents(database, "cam1", 3, 3, false, "", false)
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

	unlabeled, total, err := db.PageMotionEvents(database, "cam1", 0, 10, true, "", false)
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

	events, _, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "", false)
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
	results, total, err := db.PageMotionEvents(database, "cam1", 0, 10, false, "pessoa", false)
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
	all, totalAll, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "", false)
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

func TestBulkDeleteMotionEvents_DeletesAndReturnsFramePaths(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		ev := db.MotionEvent{
			CameraID:   "cam1",
			OccurredAt: base.Add(time.Duration(i) * time.Second),
			Score:      0.5,
			FramePath:  "frame_" + time.Duration(i).String() + ".jpg",
		}
		if err := db.InsertMotionEvent(database, ev); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	events, _, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "", false)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	ids := []int64{events[0].ID, events[1].ID}

	deleted, snaps, err := db.BulkDeleteMotionEvents(database, ids)
	if err != nil {
		t.Fatalf("BulkDeleteMotionEvents: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected deleted=2, got %d", deleted)
	}
	if len(snaps) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snaps))
	}
	for _, sn := range snaps {
		if sn.CameraID != "cam1" || sn.FramePath == "" {
			t.Errorf("missing fields in snapshot: %+v", sn)
		}
	}

	remaining, _, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "", false)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestBulkDeleteMotionEvents_EmptyIDsReturnsZero(t *testing.T) {
	database := openTestDB(t)
	deleted, snaps, err := db.BulkDeleteMotionEvents(database, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 || len(snaps) != 0 {
		t.Errorf("expected zero results, got deleted=%d snaps=%d", deleted, len(snaps))
	}
}

func TestBulkUpdateMotionEventLabels_AppliesAndClears(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		insertTestEvent(t, database, "cam1", base.Add(time.Duration(i)*time.Second), 0.5, "")
	}
	events, _, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "", false)
	ids := []int64{events[0].ID, events[1].ID}

	updated, err := db.BulkUpdateMotionEventLabels(database, ids, "cat")
	if err != nil {
		t.Fatalf("BulkUpdateMotionEventLabels: %v", err)
	}
	if updated != 2 {
		t.Errorf("expected updated=2, got %d", updated)
	}

	results, _, _ := db.PageMotionEvents(database, "cam1", 0, 10, false, "cat", false)
	if len(results) != 2 {
		t.Errorf("expected 2 events labeled cat, got %d", len(results))
	}

	// clear labels
	cleared, err := db.BulkUpdateMotionEventLabels(database, ids, "")
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared != 2 {
		t.Errorf("expected cleared=2, got %d", cleared)
	}
	results, _, _ = db.PageMotionEvents(database, "cam1", 0, 10, true, "", false)
	if len(results) != 3 {
		t.Errorf("expected all 3 to be unlabeled, got %d", len(results))
	}
}

func TestBulkUpdateMotionEventLabels_EmptyIDsReturnsZero(t *testing.T) {
	database := openTestDB(t)
	updated, err := db.BulkUpdateMotionEventLabels(database, nil, "cat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != 0 {
		t.Errorf("expected updated=0, got %d", updated)
	}
}

func TestBulkDismissMotionEvents_SetsDismissed(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(time.Second), 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(2*time.Second), 0.5, "")

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Hour))
	ids := []int64{events[0].ID, events[1].ID}

	n, err := db.BulkDismissMotionEvents(database, ids)
	if err != nil {
		t.Fatalf("BulkDismissMotionEvents: %v", err)
	}
	if n != 2 {
		t.Errorf("expected dismissed=2, got %d", n)
	}

	ev0, _ := db.GetMotionEventByID(database, events[0].ID)
	if !ev0.Dismissed {
		t.Error("event[0] deve estar dismissed=true")
	}
	ev2, _ := db.GetMotionEventByID(database, events[2].ID)
	if ev2.Dismissed {
		t.Error("event[2] não deve estar dismissed")
	}
}

func TestBulkDismissMotionEvents_EmptyIDsReturnsZero(t *testing.T) {
	database := openTestDB(t)
	n, err := db.BulkDismissMotionEvents(database, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestPageMotionEvents_ExcludesDismissedByDefault(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(time.Second), 0.5, "")

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Hour))
	db.BulkDismissMotionEvents(database, []int64{events[0].ID})

	got, total, err := db.PageMotionEvents(database, "cam1", 0, 10, false, "", false)
	if err != nil {
		t.Fatalf("PageMotionEvents: %v", err)
	}
	if total != 1 {
		t.Errorf("default view deve excluir dismissed: esperado total=1, got %d", total)
	}
	if len(got) != 1 {
		t.Fatalf("esperado 1 evento, got %d", len(got))
	}
	if got[0].ID == events[0].ID {
		t.Error("evento dismissed não deve aparecer na listagem padrão")
	}
}

func TestPageMotionEvents_ShowsDismissedWhenFiltered(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")
	insertTestEvent(t, database, "cam1", base.Add(time.Second), 0.5, "")

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Hour))
	db.BulkDismissMotionEvents(database, []int64{events[0].ID})

	got, total, err := db.PageMotionEvents(database, "cam1", 0, 10, false, "", true)
	if err != nil {
		t.Fatalf("PageMotionEvents dismissed: %v", err)
	}
	if total != 1 {
		t.Errorf("esperado total=1 dismissed, got %d", total)
	}
	if len(got) != 1 || got[0].ID != events[0].ID {
		t.Error("deve retornar o evento dismissed")
	}
}

func TestUpdateMotionEventFramePath_SetsPath(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	insertTestEvent(t, database, "cam1", base, 0.5, "")

	events, _ := db.ListMotionEvents(database, "cam1", base, base.Add(time.Second))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	id := events[0].ID

	if err := db.UpdateMotionEventFramePath(database, id, "/recordings/cam1/2026/06/01/snap.jpg"); err != nil {
		t.Fatalf("UpdateMotionEventFramePath: %v", err)
	}

	got, err := db.GetMotionEventByID(database, id)
	if err != nil {
		t.Fatalf("GetMotionEventByID: %v", err)
	}
	if got.FramePath != "/recordings/cam1/2026/06/01/snap.jpg" {
		t.Errorf("expected updated frame_path, got %q", got.FramePath)
	}
}

func TestUpdateMotionEventFramePath_UnknownIDReturnsError(t *testing.T) {
	database := openTestDB(t)

	err := db.UpdateMotionEventFramePath(database, 9999, "/some/path.jpg")
	if err == nil {
		t.Fatal("expected error for unknown event id, got nil")
	}
}

func TestListOrphanedMotionEvents_OnlyOldUncovered(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")

	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	cutoff := now.Add(-24 * time.Hour)

	// Old event covered by a recording → NOT orphan.
	if err := db.InsertRecording(database, db.Recording{
		CameraID:  "cam1",
		StartedAt: old.Add(-time.Minute),
		EndedAt:   old.Add(time.Minute),
		Path:      "/x/covered.mp4",
	}); err != nil {
		t.Fatalf("InsertRecording: %v", err)
	}
	insertTestEvent(t, database, "cam1", old, 0.5, "") // covered

	// Old event with no covering recording → orphan.
	insertTestEvent(t, database, "cam1", old.Add(30*time.Minute), 0.6, "")

	// Recent uncovered event → within retention, NOT orphan.
	insertTestEvent(t, database, "cam1", recent, 0.6, "")

	orphans, err := db.ListOrphanedMotionEvents(database, cutoff)
	if err != nil {
		t.Fatalf("ListOrphanedMotionEvents: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	want := old.Add(30 * time.Minute).Format(time.RFC3339)
	if got := orphans[0].OccurredAt.UTC().Format(time.RFC3339); got != want {
		t.Errorf("orphan occurred_at = %s, want %s", got, want)
	}
}
