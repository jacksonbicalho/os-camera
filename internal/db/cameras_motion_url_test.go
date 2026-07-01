package db_test

import (
	"testing"

	"camera/internal/db"
)

// TestCameraMotionRTSPURL_RoundTrip verifies the optional per-camera motion URL
// survives create → get → list → update.
func TestCameraMotionRTSPURL_RoundTrip(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("sub")
	cam.MotionRTSPURL = "rtsp://localhost/sub?subtype=1"
	created, err := db.CreateCamera(database, cam, nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, created.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.MotionRTSPURL != "rtsp://localhost/sub?subtype=1" {
		t.Errorf("GetCamera MotionRTSPURL = %q, want the persisted substream URL", got.MotionRTSPURL)
	}

	list, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(list) != 1 || list[0].MotionRTSPURL != "rtsp://localhost/sub?subtype=1" {
		t.Errorf("ListCameras MotionRTSPURL not round-tripped: %+v", list)
	}

	updated := got
	updated.MotionRTSPURL = "rtsp://localhost/other?subtype=1"
	if err := db.UpdateCamera(database, updated, nil); err != nil {
		t.Fatalf("UpdateCamera: %v", err)
	}
	after, _ := db.GetCamera(database, created.ID)
	if after.MotionRTSPURL != "rtsp://localhost/other?subtype=1" {
		t.Errorf("after UpdateCamera MotionRTSPURL = %q, want updated value", after.MotionRTSPURL)
	}
}

// TestCameraMotionRTSPURL_DefaultsEmpty verifies a camera without a motion URL
// round-trips as empty (motion then falls back to the main URL).
func TestCameraMotionRTSPURL_DefaultsEmpty(t *testing.T) {
	database := openTestDB(t)

	created, err := db.CreateCamera(database, makeCamera("main"), nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}
	got, _ := db.GetCamera(database, created.ID)
	if got.MotionRTSPURL != "" {
		t.Errorf("MotionRTSPURL = %q, want empty by default", got.MotionRTSPURL)
	}
}
