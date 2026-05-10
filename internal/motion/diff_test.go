package motion

import (
	"testing"

	"camera/internal/zones"
)

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

// Frame 2×2 (w=2, h=2), pixels indexados:
//   [0,0]=i0  [1,0]=i1
//   [0,1]=i2  [1,1]=i3

func TestDiffFramesMaskedNoZonesEqualsUnmasked(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	want := diffFrames(a, b)
	got := diffFramesMasked(a, b, 2, 2, nil)
	if got != want {
		t.Errorf("expected %f, got %f", want, got)
	}
}

func TestDiffFramesMaskedFullFrameExcluded(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 255, 255}
	z := []zones.Zone{{X: 0, Y: 0, W: 1, H: 1}}
	got := diffFramesMasked(a, b, 2, 2, z)
	if got != 0.0 {
		t.Errorf("expected 0.0 with full exclusion, got %f", got)
	}
}

func TestDiffFramesMaskedHalfFrameExcluded(t *testing.T) {
	// Frame 2×2: linha superior (y=0) tem diff máximo, linha inferior (y=1) tem diff zero.
	// Mascarar apenas a linha superior → score deve ser 0.
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	z := []zones.Zone{{X: 0, Y: 0, W: 1, H: 0.5}} // top 50%
	got := diffFramesMasked(a, b, 2, 2, z)
	if got != 0.0 {
		t.Errorf("expected 0.0 with top half excluded, got %f", got)
	}
}

func TestDiffFramesMaskedDenominatorExcludesmaskedPixels(t *testing.T) {
	// Frame 4×1 (w=4, h=1). Pixels: 0,1,2,3.
	// a=[0,0,0,0], b=[255,255,0,0] → sem máscara score=0.5
	// Mascarar pixels 2 e 3 (x=0.5..1.0) → apenas pixels 0 e 1 contam → score=1.0
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	z := []zones.Zone{{X: 0.5, Y: 0, W: 0.5, H: 1}}
	got := diffFramesMasked(a, b, 4, 1, z)
	if got < 0.99 || got > 1.01 {
		t.Errorf("expected ~1.0 with right half excluded, got %f", got)
	}
}

func TestDiffFramesMaskedAllPixelsMaskedReturnsZero(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 255, 255}
	z := []zones.Zone{{X: 0, Y: 0, W: 1, H: 1}}
	got := diffFramesMasked(a, b, 2, 2, z)
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}
