package db_test

import (
	"testing"

	"camera/internal/db"
)

// TestCameraLiveTransport_RoundTrip verifies the per-camera live_transport
// preference persists through create → get → list → update.
func TestCameraLiveTransport_RoundTrip(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("lt")
	cam.LiveTransport = "hls"
	created, err := db.CreateCamera(database, cam, nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, created.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.LiveTransport != "hls" {
		t.Errorf("GetCamera LiveTransport = %q, want hls", got.LiveTransport)
	}

	list, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(list) != 1 || list[0].LiveTransport != "hls" {
		t.Errorf("ListCameras LiveTransport not round-tripped: %+v", list)
	}

	updated := got
	updated.LiveTransport = "webrtc"
	if err := db.UpdateCamera(database, updated, nil); err != nil {
		t.Fatalf("UpdateCamera: %v", err)
	}
	after, err := db.GetCamera(database, created.ID)
	if err != nil {
		t.Fatalf("GetCamera after update: %v", err)
	}
	if after.LiveTransport != "webrtc" {
		t.Errorf("after UpdateCamera LiveTransport = %q, want webrtc", after.LiveTransport)
	}
}

// TestCameraLiveTransport_DefaultsAuto verifies a camera created without a
// transport preference reads back as "auto".
func TestCameraLiveTransport_DefaultsAuto(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("def")
	created, err := db.CreateCamera(database, cam, nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}
	got, err := db.GetCamera(database, created.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.LiveTransport != "auto" {
		t.Errorf("default LiveTransport = %q, want auto", got.LiveTransport)
	}
}
