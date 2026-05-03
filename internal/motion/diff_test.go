package motion

import "testing"

func TestDiffFramesIdenticalReturnsZero(t *testing.T) {
	a := []byte{100, 150, 200, 50}
	b := []byte{100, 150, 200, 50}
	if got := diffFrames(a, b); got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestDiffFramesAllChangedReturnsOne(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 255, 255}
	got := diffFrames(a, b)
	if got < 0.99 || got > 1.0 {
		t.Errorf("expected ~1.0, got %f", got)
	}
}

func TestDiffFramesHalfChangedReturnsHalf(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	got := diffFrames(a, b)
	// 2 pixels fully changed out of 4 → 0.5
	if got < 0.49 || got > 0.51 {
		t.Errorf("expected ~0.5, got %f", got)
	}
}

func TestDiffFramesDifferentLengthsPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched lengths")
		}
	}()
	diffFrames([]byte{1, 2}, []byte{1, 2, 3})
}
