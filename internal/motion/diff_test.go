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

// --- computeBBox ---

// Frame 4×4, nenhum pixel diferente → bbox cobre frame inteiro
func TestComputeBBoxNoDiff(t *testing.T) {
	frame := make([]byte, 16)
	got := computeBBox(frame, frame, 4, 4, nil)
	want := BBox{X: 0, Y: 0, W: 1, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// Frame 4×4, todos os pixels diferentes → bbox cobre frame inteiro
func TestComputeBBoxAllDiff(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	for i := range b {
		b[i] = 255
	}
	got := computeBBox(a, b, 4, 4, nil)
	want := BBox{X: 0, Y: 0, W: 1, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// Frame 4×4, apenas o pixel (1,1) é diferente (centro-esquerda)
// minX=1, minY=1, maxX=1, maxY=1
// bbox normalizado: x=1/4, y=1/4, w=1/4, h=1/4
func TestComputeBBoxSinglePixel(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	// pixel (px=1, py=1) → índice = py*w + px = 1*4+1 = 5
	b[5] = 200 // diff = 200 > threshold
	got := computeBBox(a, b, 4, 4, nil)
	want := BBox{X: 0.25, Y: 0.25, W: 0.25, H: 0.25}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// Frame 4×4, região [col 1..2, row 1..2] → bbox deve cobrir exatamente essa região
// pixels: (1,1)=5, (2,1)=6, (1,2)=9, (2,2)=10
// bbox normalizado: x=1/4, y=1/4, w=2/4=0.5, h=2/4=0.5
func TestComputeBBoxRegion(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	for _, idx := range []int{5, 6, 9, 10} {
		b[idx] = 200
	}
	got := computeBBox(a, b, 4, 4, nil)
	want := BBox{X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// Frame 4×1, apenas o pixel mais à direita (px=3) é diferente
// bbox: x=3/4, y=0, w=1/4, h=1
func TestComputeBBoxRightEdge(t *testing.T) {
	a := make([]byte, 4)
	b := make([]byte, 4)
	b[3] = 200
	got := computeBBox(a, b, 4, 1, nil)
	want := BBox{X: 0.75, Y: 0, W: 0.25, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// Pixels mascarados não devem contar para o bbox
// Frame 4×1: pixel 0 mascarado (diff alto) + pixel 3 não mascarado (diff alto)
// → bbox deve ser apenas o pixel 3
func TestComputeBBoxIgnoresMaskedPixels(t *testing.T) {
	a := make([]byte, 4)
	b := make([]byte, 4)
	b[0] = 200 // mascarado
	b[3] = 200 // não mascarado
	z := []zones.Zone{{X: 0, Y: 0, W: 0.5, H: 1}} // mascara pixels 0 e 1
	got := computeBBox(a, b, 4, 1, z)
	want := BBox{X: 0.75, Y: 0, W: 0.25, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}
