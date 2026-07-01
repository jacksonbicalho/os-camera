package motion

import (
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/ffprobe"
)

// TestNewUsesMotionSubstreamURL verifies the Monitor reads the per-camera motion
// URL (e.g. a lighter substream) when set, instead of the main RTSP URL.
func TestNewUsesMotionSubstreamURL(t *testing.T) {
	cam := config.CameraConfig{
		ID:            "cam",
		RTSPURL:       "rtsp://cam/main?subtype=0",
		MotionRTSPURL: "rtsp://cam/sub?subtype=1",
	}
	stream := ffprobe.StreamInfo{Width: 640, Height: 480}
	cfg := config.MotionConfig{Enabled: true, FPS: 2, Threshold: 0.02}

	mon := New(cam, stream, cfg, t.TempDir(), time.Second, discardLogger(), nil, nil)

	if mon.det.url != "rtsp://cam/sub?subtype=1" {
		t.Errorf("detector url = %q, want the motion substream URL", mon.det.url)
	}
}

// TestNewFallsBackToMainURL verifies that with no motion override the Monitor
// keeps reading the main RTSP URL (no regression).
func TestNewFallsBackToMainURL(t *testing.T) {
	cam := config.CameraConfig{
		ID:      "cam",
		RTSPURL: "rtsp://cam/main?subtype=0",
	}
	stream := ffprobe.StreamInfo{Width: 1920, Height: 1080}
	cfg := config.MotionConfig{Enabled: true, FPS: 2, Threshold: 0.02}

	mon := New(cam, stream, cfg, t.TempDir(), time.Second, discardLogger(), nil, nil)

	if mon.det.url != "rtsp://cam/main?subtype=0" {
		t.Errorf("detector url = %q, want fallback to main URL", mon.det.url)
	}
}
