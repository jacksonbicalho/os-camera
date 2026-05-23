package db_test

import (
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
)

func makeCamera(name string) config.CameraConfig {
	dur5m := config.Duration(5 * time.Minute)
	dur30s := config.Duration(30 * time.Second)
	return config.CameraConfig{
		Name:              name,
		RTSPURL:           "rtsp://localhost/" + name,
		ChunkDuration:     dur5m,
		ReconnectInterval: dur30s,
		DisplayOrder:      0,
	}
}

func TestCreateCamera_GeneratesUUID(t *testing.T) {
	database := openTestDB(t)
	created, err := db.CreateCamera(database, makeCamera("portão"), nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.ID == "portão" {
		t.Error("ID should not equal Name")
	}
	if created.Name != "portão" {
		t.Errorf("Name: got %q, want %q", created.Name, "portão")
	}
}

func TestCreateCamera_SameNameDifferentIDs(t *testing.T) {
	database := openTestDB(t)
	c1, err := db.CreateCamera(database, config.CameraConfig{Name: "portão", RTSPURL: "rtsp://host/1"}, nil)
	if err != nil {
		t.Fatalf("CreateCamera 1: %v", err)
	}
	c2, err := db.CreateCamera(database, config.CameraConfig{Name: "portão", RTSPURL: "rtsp://host/2"}, nil)
	if err != nil {
		t.Fatalf("CreateCamera 2: %v", err)
	}
	if c1.ID == c2.ID {
		t.Error("expected different IDs for cameras with same name")
	}
}

func TestCreateAndGetCamera(t *testing.T) {
	database := openTestDB(t)

	cam, err := db.CreateCamera(database, makeCamera("entrada"), nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, cam.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.Name != "entrada" {
		t.Errorf("name: got %q, want %q", got.Name, "entrada")
	}
	if got.RTSPURL != "rtsp://localhost/entrada" {
		t.Errorf("rtsp_url: got %q, want %q", got.RTSPURL, "rtsp://localhost/entrada")
	}
}

func TestListCameras(t *testing.T) {
	database := openTestDB(t)

	for _, name := range []string{"cam1", "cam2", "cam3"} {
		if _, err := db.CreateCamera(database, makeCamera(name), nil); err != nil {
			t.Fatalf("CreateCamera %s: %v", name, err)
		}
	}

	cams, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(cams) != 3 {
		t.Errorf("esperava 3, got %d", len(cams))
	}
}

func TestUpdateCamera(t *testing.T) {
	database := openTestDB(t)

	cam, err := db.CreateCamera(database, makeCamera("cam"), nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	updated := cam
	updated.RTSPURL = "rtsp://novo/cam"
	updated.DisplayOrder = 5

	if err := db.UpdateCamera(database, updated, nil); err != nil {
		t.Fatalf("UpdateCamera: %v", err)
	}

	got, _ := db.GetCamera(database, cam.ID)
	if got.RTSPURL != "rtsp://novo/cam" {
		t.Errorf("rtsp_url: got %q, want %q", got.RTSPURL, "rtsp://novo/cam")
	}
	if got.DisplayOrder != 5 {
		t.Errorf("display_order: got %d, want 5", got.DisplayOrder)
	}
}

func TestDeleteCamera(t *testing.T) {
	database := openTestDB(t)

	cam, err := db.CreateCamera(database, makeCamera("cam"), nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	if err := db.DeleteCamera(database, cam.ID); err != nil {
		t.Fatalf("DeleteCamera: %v", err)
	}

	_, err = db.GetCamera(database, cam.ID)
	if err == nil {
		t.Error("esperava erro ao buscar câmera deletada")
	}
}

func TestUpdateCameraStreamInfo(t *testing.T) {
	database := openTestDB(t)

	cam, err := db.CreateCamera(database, makeCamera("cam"), nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	hasAudio := true
	if err := db.UpdateCameraStreamInfo(database, cam.ID, "h264", &hasAudio, 1920, 1080); err != nil {
		t.Fatalf("UpdateCameraStreamInfo: %v", err)
	}

	got, err := db.GetCamera(database, cam.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.VideoCodec != "h264" {
		t.Errorf("video_codec: got %q, want %q", got.VideoCodec, "h264")
	}
	if got.HasAudio == nil || !*got.HasAudio {
		t.Errorf("has_audio: got %v, want true", got.HasAudio)
	}
	if got.Width != 1920 {
		t.Errorf("width: got %d, want 1920", got.Width)
	}
	if got.Height != 1080 {
		t.Errorf("height: got %d, want 1080", got.Height)
	}
}

func TestUpdateCameraStreamInfo_SkipsZeroValues(t *testing.T) {
	database := openTestDB(t)

	baseCam := makeCamera("cam")
	baseCam.VideoCodec = "hevc"
	baseCam.Width = 2560
	baseCam.Height = 1440
	hasAudio := false
	baseCam.HasAudio = &hasAudio

	cam, err := db.CreateCamera(database, baseCam, nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	if err := db.UpdateCameraStreamInfo(database, cam.ID, "", nil, 0, 0); err != nil {
		t.Fatalf("UpdateCameraStreamInfo: %v", err)
	}

	got, err := db.GetCamera(database, cam.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.VideoCodec != "hevc" {
		t.Errorf("video_codec: got %q, want %q", got.VideoCodec, "hevc")
	}
	if got.Width != 2560 {
		t.Errorf("width: got %d, want 2560", got.Width)
	}
}

func TestListCameras_IncludesCaptureResolution(t *testing.T) {
	database := openTestDB(t)

	motion := &config.MotionConfig{
		Enabled:       true,
		Threshold:     0.02,
		FPS:           2,
		CaptureWidth:  960,
		CaptureHeight: 540,
	}
	if _, err := db.CreateCamera(database, makeCamera("cam"), motion); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	cams, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(cams) != 1 || cams[0].Motion == nil {
		t.Fatalf("esperava 1 câmera com motion, got %v", cams)
	}
	if cams[0].Motion.CaptureWidth != 960 {
		t.Errorf("CaptureWidth: got %d, want 960", cams[0].Motion.CaptureWidth)
	}
	if cams[0].Motion.CaptureHeight != 540 {
		t.Errorf("CaptureHeight: got %d, want 540", cams[0].Motion.CaptureHeight)
	}
}

func TestCreateCamera_WithMotion(t *testing.T) {
	database := openTestDB(t)

	motion := &config.MotionConfig{
		Enabled:         true,
		Threshold:       0.05,
		FPS:             4,
		CooldownSeconds: 60,
	}

	cam, err := db.CreateCamera(database, makeCamera("cam-m"), motion)
	if err != nil {
		t.Fatalf("CreateCamera com motion: %v", err)
	}

	got, err := db.GetCamera(database, cam.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.Motion == nil {
		t.Fatal("motion config não retornada")
	}
	if !got.Motion.Enabled {
		t.Error("motion.enabled deveria ser true")
	}
	if got.Motion.Threshold != 0.05 {
		t.Errorf("motion.threshold: got %v, want 0.05", got.Motion.Threshold)
	}
}

func TestMotionConfig_PlaybackLeadSeconds(t *testing.T) {
	database := openTestDB(t)

	motion := &config.MotionConfig{
		Enabled:             true,
		Threshold:           0.05,
		PlaybackLeadSeconds: 20,
	}
	cam, err := db.CreateCamera(database, makeCamera("cam-lead"), motion)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, cam.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.Motion == nil {
		t.Fatal("motion config não retornada")
	}
	if got.Motion.PlaybackLeadSeconds != 20 {
		t.Errorf("playback_lead_seconds: got %d, want 20", got.Motion.PlaybackLeadSeconds)
	}
}

func TestMotionConfig_PlaybackLeadSecondsDefault(t *testing.T) {
	database := openTestDB(t)

	motion := &config.MotionConfig{Enabled: true, Threshold: 0.05}
	cam, err := db.CreateCamera(database, makeCamera("cam-default"), motion)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, cam.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.Motion.PlaybackLeadSeconds != 10 {
		t.Errorf("playback_lead_seconds default: got %d, want 10", got.Motion.PlaybackLeadSeconds)
	}
}

func TestRecordingEnabled_PersistsTrue(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("cam")
	cam.RecordingEnabled = true
	created, err := db.CreateCamera(database, cam, nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, created.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if !got.RecordingEnabled {
		t.Error("RecordingEnabled: got false, want true")
	}
}

func TestRecordingEnabled_CanBeDisabled(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("cam")
	cam.RecordingEnabled = true
	created, err := db.CreateCamera(database, cam, nil)
	if err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	created.RecordingEnabled = false
	if err := db.UpdateCamera(database, created, nil); err != nil {
		t.Fatalf("UpdateCamera: %v", err)
	}

	got, err := db.GetCamera(database, created.ID)
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.RecordingEnabled {
		t.Error("RecordingEnabled: got true, want false")
	}
}

func TestListCameras_IncludesRecordingEnabled(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("cam")
	cam.RecordingEnabled = true
	if _, err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	cams, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(cams) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(cams))
	}
	if !cams[0].RecordingEnabled {
		t.Error("RecordingEnabled: got false, want true")
	}
}

func TestReorderCameras(t *testing.T) {
	database := openTestDB(t)

	c1, _ := db.CreateCamera(database, makeCamera("c1"), nil)
	c2, _ := db.CreateCamera(database, makeCamera("c2"), nil)
	c3, _ := db.CreateCamera(database, makeCamera("c3"), nil)

	// Reverse order: c3 first, then c1, then c2.
	if err := db.ReorderCameras(database, []string{c3.ID, c1.ID, c2.ID}); err != nil {
		t.Fatalf("ReorderCameras: %v", err)
	}

	cams, err := db.ListCameras(database)
	if err != nil {
		t.Fatalf("ListCameras: %v", err)
	}
	if len(cams) != 3 {
		t.Fatalf("expected 3 cameras, got %d", len(cams))
	}
	want := []string{c3.ID, c1.ID, c2.ID}
	for i, cam := range cams {
		if cam.ID != want[i] {
			t.Errorf("position %d: got %q, want %q", i, cam.ID, want[i])
		}
	}
}
