package motion

import (
	"testing"
	"time"
)

func TestStoreCallsOnEvent(t *testing.T) {
	var called bool
	var gotScore float64
	var gotFrame, gotLabel, gotColor string
	var gotBBox BBox

	onEvent := func(cameraID string, ts time.Time, score float64, frame, label, color string, bbox BBox) {
		called = true
		gotScore = score
		gotFrame = frame
		gotLabel = label
		gotColor = color
		gotBBox = bbox
	}

	st := newStore("/tmp", onEvent)
	ts := time.Date(2026, 5, 3, 14, 30, 0, 0, time.UTC)
	err := st.record("cam1", ts, 0.42, "20260503143000_motion.jpg", "jardim", "#3b82f6", BBox{X: 0.1, Y: 0.2, W: 0.3, H: 0.4})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if !called {
		t.Fatal("onEvent not called")
	}
	if gotScore != 0.42 {
		t.Errorf("score: got %f, want 0.42", gotScore)
	}
	if gotFrame != "20260503143000_motion.jpg" {
		t.Errorf("frame: got %q, want 20260503143000_motion.jpg", gotFrame)
	}
	if gotLabel != "jardim" {
		t.Errorf("label: got %q, want jardim", gotLabel)
	}
	if gotColor != "#3b82f6" {
		t.Errorf("color: got %q, want #3b82f6", gotColor)
	}
	if gotBBox.X != 0.1 || gotBBox.Y != 0.2 || gotBBox.W != 0.3 || gotBBox.H != 0.4 {
		t.Errorf("bbox: got %+v", gotBBox)
	}
}

func TestStoreNoOnEvent(t *testing.T) {
	st := newStore("/tmp", nil)
	ts := time.Date(2026, 5, 3, 14, 30, 0, 0, time.UTC)
	if err := st.record("cam1", ts, 0.5, "", "", "", BBox{}); err != nil {
		t.Fatalf("record with nil onEvent: %v", err)
	}
}
