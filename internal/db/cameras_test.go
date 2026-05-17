package db_test

import (
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/db"
)

func makeCamera(id string) config.CameraConfig {
	dur5m := config.Duration(5 * time.Minute)
	dur30s := config.Duration(30 * time.Second)
	return config.CameraConfig{
		ID:                id,
		RTSPURL:           "rtsp://localhost/" + id,
		ChunkDuration:     dur5m,
		ReconnectInterval: dur30s,
		DisplayOrder:      0,
	}
}

func TestCreateAndGetCamera(t *testing.T) {
	database := openTestDB(t)

	cam := makeCamera("entrada")
	if err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, "entrada")
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.ID != "entrada" {
		t.Errorf("id: got %q, want %q", got.ID, "entrada")
	}
	if got.RTSPURL != "rtsp://localhost/entrada" {
		t.Errorf("rtsp_url: got %q, want %q", got.RTSPURL, "rtsp://localhost/entrada")
	}
}

func TestListCameras(t *testing.T) {
	database := openTestDB(t)

	for _, id := range []string{"cam1", "cam2", "cam3"} {
		if err := db.CreateCamera(database, makeCamera(id), nil); err != nil {
			t.Fatalf("CreateCamera %s: %v", id, err)
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

	if err := db.CreateCamera(database, makeCamera("cam"), nil); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	updated := makeCamera("cam")
	updated.RTSPURL = "rtsp://novo/cam"
	updated.DisplayOrder = 5

	if err := db.UpdateCamera(database, updated, nil); err != nil {
		t.Fatalf("UpdateCamera: %v", err)
	}

	got, _ := db.GetCamera(database, "cam")
	if got.RTSPURL != "rtsp://novo/cam" {
		t.Errorf("rtsp_url: got %q, want %q", got.RTSPURL, "rtsp://novo/cam")
	}
	if got.DisplayOrder != 5 {
		t.Errorf("display_order: got %d, want 5", got.DisplayOrder)
	}
}

func TestDeleteCamera(t *testing.T) {
	database := openTestDB(t)

	if err := db.CreateCamera(database, makeCamera("cam"), nil); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	if err := db.DeleteCamera(database, "cam"); err != nil {
		t.Fatalf("DeleteCamera: %v", err)
	}

	_, err := db.GetCamera(database, "cam")
	if err == nil {
		t.Error("esperava erro ao buscar câmera deletada")
	}
}

func TestUpdateCameraStreamInfo(t *testing.T) {
	database := openTestDB(t)

	// Camera criada sem valores de stream (tudo "auto")
	if err := db.CreateCamera(database, makeCamera("cam"), nil); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	hasAudio := true
	if err := db.UpdateCameraStreamInfo(database, "cam", "h264", &hasAudio, 1920, 1080); err != nil {
		t.Fatalf("UpdateCameraStreamInfo: %v", err)
	}

	got, err := db.GetCamera(database, "cam")
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

	// Camera criada com codec e resolução explícitos
	cam := makeCamera("cam")
	cam.VideoCodec = "hevc"
	cam.Width = 2560
	cam.Height = 1440
	hasAudio := false
	cam.HasAudio = &hasAudio
	if err := db.CreateCamera(database, cam, nil); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	// Probe retornou valores zerados (falhou) — não deve sobrescrever
	if err := db.UpdateCameraStreamInfo(database, "cam", "", nil, 0, 0); err != nil {
		t.Fatalf("UpdateCameraStreamInfo: %v", err)
	}

	got, err := db.GetCamera(database, "cam")
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
	if err := db.CreateCamera(database, makeCamera("cam"), motion); err != nil {
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

	if err := db.CreateCamera(database, makeCamera("cam-m"), motion); err != nil {
		t.Fatalf("CreateCamera com motion: %v", err)
	}

	got, err := db.GetCamera(database, "cam-m")
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
		Enabled:              true,
		Threshold:            0.05,
		PlaybackLeadSeconds:  20,
	}
	if err := db.CreateCamera(database, makeCamera("cam-lead"), motion); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, "cam-lead")
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
	if err := db.CreateCamera(database, makeCamera("cam-default"), motion); err != nil {
		t.Fatalf("CreateCamera: %v", err)
	}

	got, err := db.GetCamera(database, "cam-default")
	if err != nil {
		t.Fatalf("GetCamera: %v", err)
	}
	if got.Motion.PlaybackLeadSeconds != 10 {
		t.Errorf("playback_lead_seconds default: got %d, want 10", got.Motion.PlaybackLeadSeconds)
	}
}
