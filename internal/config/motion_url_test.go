package config_test

import (
	"testing"

	"camera/internal/config"
)

func TestEffectiveMotionURLFallsBackToMainURL(t *testing.T) {
	cam := config.CameraConfig{RTSPURL: "rtsp://main/stream"}
	if got := cam.EffectiveMotionURL(); got != "rtsp://main/stream" {
		t.Errorf("EffectiveMotionURL() = %q, want fallback to main rtsp_url", got)
	}
}

func TestEffectiveMotionURLUsesOverrideWhenSet(t *testing.T) {
	cam := config.CameraConfig{
		RTSPURL:       "rtsp://main/stream",
		MotionRTSPURL: "rtsp://main/substream",
	}
	if got := cam.EffectiveMotionURL(); got != "rtsp://main/substream" {
		t.Errorf("EffectiveMotionURL() = %q, want the motion override", got)
	}
}
