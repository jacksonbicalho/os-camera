package discovery

import (
	"testing"

	"camera/internal/webcam"
)

func TestWebcamResults(t *testing.T) {
	devs := []webcam.Device{
		{Index: 0, Path: "/dev/video0", Name: "Integrated Camera", RTSPName: "webcam0"},
		{Index: 2, Path: "/dev/video2", Name: "USB Cam", RTSPName: "webcam2"},
	}
	got := webcamResults(devs, "127.0.0.1:8554")
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	r := got[0]
	if r.Kind != "webcam" || r.Name != "Integrated Camera" || r.IP != "127.0.0.1" || r.Port != 8554 {
		t.Fatalf("unexpected result: %+v", r)
	}
	if len(r.RTSPURLs) != 1 || r.RTSPURLs[0] != "rtsp://127.0.0.1:8554/webcam0" {
		t.Fatalf("unexpected rtsp: %+v", r.RTSPURLs)
	}
	if got[1].RTSPURLs[0] != "rtsp://127.0.0.1:8554/webcam2" {
		t.Fatalf("unexpected rtsp[1]: %+v", got[1].RTSPURLs)
	}
}

func TestWebcamResults_Empty(t *testing.T) {
	if got := webcamResults(nil, "127.0.0.1:8554"); len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}
