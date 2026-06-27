package webcam_test

import (
	"testing"
	"testing/fstest"

	"camera/internal/webcam"
)

func TestList_DedupeByNameAndOrder(t *testing.T) {
	root := fstest.MapFS{
		"video0/name": {Data: []byte("Integrated Camera: Integrated C\n")},
		"video1/name": {Data: []byte("Integrated Camera: Integrated C\n")}, // mesmo nome → dedup
		"video2/name": {Data: []byte("USB Cam\n")},
		"notavideo/x": {Data: []byte("ignore")},
	}

	got := webcam.List(root)
	if len(got) != 2 {
		t.Fatalf("expected 2 devices, got %d: %+v", len(got), got)
	}
	// menor índice por nome, ordenado por índice
	if got[0].Path != "/dev/video0" || got[0].RTSPName != "webcam0" || got[0].Name != "Integrated Camera: Integrated C" {
		t.Errorf("device 0 inesperado: %+v", got[0])
	}
	if got[1].Path != "/dev/video2" || got[1].RTSPName != "webcam2" || got[1].Name != "USB Cam" {
		t.Errorf("device 1 inesperado: %+v", got[1])
	}
}

func TestList_EmptyWhenNoSysfs(t *testing.T) {
	if got := webcam.List(fstest.MapFS{}); len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}

func TestRTSPURL(t *testing.T) {
	if got := webcam.RTSPURL("127.0.0.1:8554", "webcam0"); got != "rtsp://127.0.0.1:8554/webcam0" {
		t.Fatalf("RTSPURL: %s", got)
	}
}
