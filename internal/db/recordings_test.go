package db_test

import (
	"testing"
	"time"

	"camera/internal/db"
)

func TestLastRecordingPerCamera_ReturnsLatestPerCamera(t *testing.T) {
	database := openTestDB(t)
	ensureCamera(t, database, "cam1")
	ensureCamera(t, database, "cam2")

	base := time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)
	db.InsertRecording(database, db.Recording{CameraID: "cam1", StartedAt: base, Path: "a.mp4"})
	db.InsertRecording(database, db.Recording{CameraID: "cam1", StartedAt: base.Add(5 * time.Minute), Path: "b.mp4"})
	db.InsertRecording(database, db.Recording{CameraID: "cam2", StartedAt: base.Add(2 * time.Minute), Path: "c.mp4"})

	result, err := db.LastRecordingPerCamera(database)
	if err != nil {
		t.Fatalf("LastRecordingPerCamera: %v", err)
	}
	want1 := base.Add(5 * time.Minute)
	if !result["cam1"].Equal(want1) {
		t.Errorf("cam1: want %v, got %v", want1, result["cam1"])
	}
	want2 := base.Add(2 * time.Minute)
	if !result["cam2"].Equal(want2) {
		t.Errorf("cam2: want %v, got %v", want2, result["cam2"])
	}
}

func TestLastRecordingPerCamera_ReturnsEmptyMapWhenNoRecordings(t *testing.T) {
	database := openTestDB(t)

	result, err := db.LastRecordingPerCamera(database)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}
