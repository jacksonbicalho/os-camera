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
	got, found := computeBBox(frame, frame, 4, 4, nil)
	want := BBox{X: 0, Y: 0, W: 1, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
	if found {
		t.Error("expected found=false when no pixels differ")
	}
}

// Frame 4×4, todos os pixels diferentes → bbox cobre frame inteiro
func TestComputeBBoxAllDiff(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	for i := range b {
		b[i] = 255
	}
	got, found := computeBBox(a, b, 4, 4, nil)
	want := BBox{X: 0, Y: 0, W: 1, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
	if !found {
		t.Error("expected found=true when all pixels differ")
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
	got, _ := computeBBox(a, b, 4, 4, nil)
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
	got, _ := computeBBox(a, b, 4, 4, nil)
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
	got, _ := computeBBox(a, b, 4, 1, nil)
	want := BBox{X: 0.75, Y: 0, W: 0.25, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// --- diffFramesForZone ---

// Frame 2×2, zona cobre tudo → igual ao diff sem máscara
func TestDiffFramesForZoneFullFrame(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 255, 255}
	z := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	got := diffFramesForZone(a, b, 2, 2, z)
	if got < 0.99 || got > 1.01 {
		t.Errorf("expected ~1.0, got %f", got)
	}
}

// Frame 4×1, zona cobre apenas os dois primeiros pixels (x=0..0.5)
// a=[0,0,0,0], b=[255,255,0,0] → zona contém pixels 0 e 1 (diff=255) → score=1.0
func TestDiffFramesForZoneLeftHalf(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	z := zones.Zone{X: 0, Y: 0, W: 0.5, H: 1}
	got := diffFramesForZone(a, b, 4, 1, z)
	if got < 0.99 || got > 1.01 {
		t.Errorf("expected ~1.0, got %f", got)
	}
}

// Frame 4×1, zona cobre apenas os dois últimos pixels (x=0.5..1.0) sem diff → score=0
func TestDiffFramesForZoneRightHalfNoDiff(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	z := zones.Zone{X: 0.5, Y: 0, W: 0.5, H: 1}
	got := diffFramesForZone(a, b, 4, 1, z)
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

// Frame vazio → score=0 (sem pânico)
func TestDiffFramesForZoneEmptyFrame(t *testing.T) {
	z := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	got := diffFramesForZone([]byte{}, []byte{}, 0, 0, z)
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

// Comprimentos diferentes → pânico
func TestDiffFramesForZoneMismatchPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for mismatched lengths")
		}
	}()
	z := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	diffFramesForZone([]byte{1, 2}, []byte{1}, 2, 1, z)
}

// --- downsampleAvg ---

// Frame 4×2, scale=0.5 → 2×1, cada pixel é média de bloco 2×2
func TestDownsampleAvgHalf(t *testing.T) {
	// 4×2 com todos os pixels = 100
	buf := make([]byte, 8)
	for i := range buf {
		buf[i] = 100
	}
	out, w, h := downsampleAvg(buf, 4, 2, 0.5)
	if w != 2 || h != 1 {
		t.Fatalf("expected 2×1, got %d×%d", w, h)
	}
	for _, v := range out {
		if v != 100 {
			t.Errorf("expected 100, got %d", v)
		}
	}
}

// Scale=1 → sem mudança
func TestDownsampleAvgScale1(t *testing.T) {
	buf := []byte{10, 20, 30, 40}
	out, w, h := downsampleAvg(buf, 2, 2, 1.0)
	if w != 2 || h != 2 || len(out) != 4 {
		t.Fatalf("expected 2×2, got %d×%d len=%d", w, h, len(out))
	}
}

// Scale=0 → sem mudança (tratado como sem downscale)
func TestDownsampleAvgScale0(t *testing.T) {
	buf := []byte{10, 20, 30, 40}
	out, w, h := downsampleAvg(buf, 2, 2, 0)
	if w != 2 || h != 2 || len(out) != 4 {
		t.Fatalf("expected 2×2, got %d×%d len=%d", w, h, len(out))
	}
}

// diffFramesForZoneScaled com scale=1 deve coincidir com diffFramesForZone
func TestDiffFramesForZoneScaledScale1EqualsFull(t *testing.T) {
	a := []byte{0, 0, 0, 0}
	b := []byte{255, 255, 0, 0}
	z := zones.Zone{X: 0, Y: 0, W: 0.5, H: 1, Scale: 1.0}
	want := diffFramesForZone(a, b, 4, 1, z)
	got := diffFramesForZoneScaled(a, b, 4, 1, z)
	if got != want {
		t.Errorf("expected %f, got %f", want, got)
	}
}

// Com scale=0.5 o resultado ainda detecta diff (apenas menos pixels)
func TestDiffFramesForZoneScaledDetectsMotion(t *testing.T) {
	// Frame 4×4, zona cobre tudo, a=0 b=255 → diff=1.0 independente de scale
	a := make([]byte, 16)
	b := make([]byte, 16)
	for i := range b {
		b[i] = 255
	}
	z := zones.Zone{X: 0, Y: 0, W: 1, H: 1, Scale: 0.5}
	got := diffFramesForZoneScaled(a, b, 4, 4, z)
	if got < 0.99 || got > 1.01 {
		t.Errorf("expected ~1.0, got %f", got)
	}
}

// --- computeBBox ---

// Pixels com diff típico de artefato de I-frame H.265 (≤ 20) não devem
// expandir o bbox — devem retornar o fallback de frame inteiro.
func TestComputeBBoxFiltersCodecArtifact(t *testing.T) {
	a := make([]byte, 16) // 4×4
	b := make([]byte, 16)
	// diff=15: faixa típica de artefato de I-frame H.265 (10–20 unidades)
	b[5] = 15
	got, found := computeBBox(a, b, 4, 4, nil)
	want := BBox{X: 0, Y: 0, W: 1, H: 1} // fallback = frame inteiro
	if got != want {
		t.Errorf("artefato de codec expandiu o bbox: got %+v, want fallback %+v", got, want)
	}
	if found {
		t.Error("expected found=false para artefato abaixo do threshold")
	}
}

// Pixels com diff acima do bboxPixelThreshold (movimento real) devem
// produzir um bbox preciso.
func TestComputeBBoxRealMotion(t *testing.T) {
	a := make([]byte, 16) // 4×4
	b := make([]byte, 16)
	// diff=30: bordas de objeto em movimento real (gato, pessoa → 30–150+)
	b[5] = 30 // pixel (px=1, py=1)
	got, found := computeBBox(a, b, 4, 4, nil)
	want := BBox{X: 0.25, Y: 0.25, W: 0.25, H: 0.25}
	if got != want {
		t.Errorf("movimento real não produziu bbox preciso: got %+v, want %+v", got, want)
	}
	if !found {
		t.Error("expected found=true para movimento real acima do threshold")
	}
}

// --- computeBBoxInZone ---

// Movimento sutil dentro de zona (diff=15, típico de gato à noite em câmera IR)
// deve produzir bbox preciso — não o fallback da zona inteira.
// diff=15 < bboxPixelThreshold=25 (global), mas ≥ zoneBboxPixelThreshold=12.
func TestComputeBBoxInZoneSubtleMotion(t *testing.T) {
	// Frame 8×8, zona cobre toda a área
	// Movimento apenas no pixel (px=4, py=4), diff=15
	a := make([]byte, 64)
	b := make([]byte, 64)
	b[4*8+4] = 15 // pixel (px=4, py=4)
	z := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	got, found := computeBBoxInZone(a, b, 8, 8, z)
	want := BBox{X: 0.5, Y: 0.5, W: 0.125, H: 0.125}
	if got != want {
		t.Errorf("movimento sutil na zona retornou fallback em vez de bbox preciso: got %+v, want %+v", got, want)
	}
	if !found {
		t.Error("expected found=true para movimento sutil acima de zoneBboxPixelThreshold")
	}
}

// Retorna o bbox do objeto em movimento dentro da zona (não o bbox da zona inteira)
func TestComputeBBoxInZoneLocalizesMotion(t *testing.T) {
	// Frame 8×8, zona cobre a metade inferior (y=0.5..1.0)
	// Movimento apenas no pixel (px=4, py=5)
	a := make([]byte, 64)
	b := make([]byte, 64)
	b[5*8+4] = 200 // i=44
	z := zones.Zone{X: 0, Y: 0.5, W: 1, H: 0.5}
	got, _ := computeBBoxInZone(a, b, 8, 8, z)
	want := BBox{X: 0.5, Y: 0.625, W: 0.125, H: 0.125}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// Sem pixels ativos dentro da zona → retorna o bbox da zona como fallback e found=false
func TestComputeBBoxInZoneNoActivePixels(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	z := zones.Zone{X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
	got, found := computeBBoxInZone(a, b, 4, 4, z)
	want := BBox{X: z.X, Y: z.Y, W: z.W, H: z.H}
	if got != want {
		t.Errorf("expected zone fallback %+v, got %+v", want, got)
	}
	if found {
		t.Error("expected found=false quando nenhum pixel ativo na zona")
	}
}

// Pixel ativo fora da zona não deve influenciar o bbox
func TestComputeBBoxInZoneIgnoresPixelsOutsideZone(t *testing.T) {
	a := make([]byte, 16)
	b := make([]byte, 16)
	b[0] = 200 // pixel (0,0), fora da zona
	z := zones.Zone{X: 0.25, Y: 0.25, W: 0.75, H: 0.75}
	got, _ := computeBBoxInZone(a, b, 4, 4, z)
	want := BBox{X: z.X, Y: z.Y, W: z.W, H: z.H}
	if got != want {
		t.Errorf("expected zone fallback %+v, got %+v", want, got)
	}
}

// Pixels mascarados não devem contar para o bbox
// Frame 4×1: pixel 0 mascarado (diff alto) + pixel 3 não mascarado (diff alto)
// → bbox deve ser apenas o pixel 3
func TestComputeBBoxIgnoresMaskedPixels(t *testing.T) {
	a := make([]byte, 4)
	b := make([]byte, 4)
	b[0] = 200                                    // mascarado
	b[3] = 200                                    // não mascarado
	z := []zones.Zone{{X: 0, Y: 0, W: 0.5, H: 1}} // mascara pixels 0 e 1
	got, _ := computeBBox(a, b, 4, 1, z)
	want := BBox{X: 0.75, Y: 0, W: 0.25, H: 1}
	if got != want {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

// --- density filtering (anti-sparse-noise) ---

// 16×16 frame with diff at 4 corner pixels only. With connected components each
// corner becomes its own 1-pixel blob with a tight 1/16×1/16 bbox — much better
// than the old min/max which spanned the entire frame. found=true; bbox is tiny.
func TestComputeBBoxRejectsSparseNoise(t *testing.T) {
	a := make([]byte, 256)
	b := make([]byte, 256)
	b[0] = 200        // (0,0)
	b[15] = 200       // (15,0)
	b[15*16] = 200    // (0,15)
	b[15*16+15] = 200 // (15,15)
	got, found := computeBBox(a, b, 16, 16, nil)
	if !found {
		t.Fatal("expected found=true: each corner is its own valid 1-pixel blob")
	}
	// Must be a tiny bbox (single pixel = 1/16 of the frame), not the whole frame.
	if got.W > 0.1 || got.H > 0.1 {
		t.Errorf("expected tiny bbox for isolated corner pixel, got %+v", got)
	}
}

// 16×16 frame with 4×4 dense cluster in center. Density inside bbox is 100% so
// the bbox is preserved with correct dimensions.
func TestComputeBBoxKeepsDenseCluster(t *testing.T) {
	a := make([]byte, 256)
	b := make([]byte, 256)
	for y := 6; y < 10; y++ {
		for x := 6; x < 10; x++ {
			b[y*16+x] = 200
		}
	}
	got, found := computeBBox(a, b, 16, 16, nil)
	if !found {
		t.Fatal("expected found=true for dense 4×4 cluster")
	}
	// bbox should hug the cluster: x=6/16=0.375, w=4/16=0.25
	if got.X < 0.36 || got.X > 0.39 {
		t.Errorf("expected X≈0.375, got %.3f", got.X)
	}
	if got.W < 0.24 || got.W > 0.26 {
		t.Errorf("expected W≈0.25, got %.3f", got.W)
	}
}

// Sparse corner pixels inside a zone: connected components returns a tiny bbox
// for the best single-pixel blob, not the zone fallback.
func TestComputeBBoxInZoneRejectsSparseNoise(t *testing.T) {
	a := make([]byte, 256)
	b := make([]byte, 256)
	b[0] = 200
	b[15] = 200
	b[15*16] = 200
	b[15*16+15] = 200
	z := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	got, found := computeBBoxInZone(a, b, 16, 16, z)
	if !found {
		t.Fatal("expected found=true: each corner is its own valid 1-pixel blob")
	}
	// Must be a tiny bbox (single pixel = 1/16), not the whole zone.
	if got.W > 0.1 || got.H > 0.1 {
		t.Errorf("expected tiny bbox for isolated corner pixel, got %+v", got)
	}
}

// --- updateBackground ---

func TestUpdateBackground(t *testing.T) {
	bg := []byte{100}
	cur := []byte{200}
	updateBackground(bg, cur, 0.5)
	// bg_new = 100*0.5 + 200*0.5 = 150
	if bg[0] != 150 {
		t.Errorf("expected 150, got %d", bg[0])
	}
}

func TestUpdateBackgroundAlphaZeroNoChange(t *testing.T) {
	bg := []byte{80}
	cur := []byte{255}
	updateBackground(bg, cur, 0)
	if bg[0] != 80 {
		t.Errorf("expected no change with alpha=0, got %d", bg[0])
	}
}

func TestUpdateBackgroundAlphaOneBecomesCurrentFrame(t *testing.T) {
	bg := []byte{50, 100, 150}
	cur := []byte{10, 20, 30}
	updateBackground(bg, cur, 1.0)
	for i, want := range cur {
		if bg[i] != want {
			t.Errorf("bg[%d]: expected %d, got %d", i, want, bg[i])
		}
	}
}

// TestComputeBBoxBackgroundSubtractionLocalizesObject demonstrates that diffing
// against a background frame (instead of prev) correctly localizes the object's
// CURRENT position when old and new positions are adjacent (one continuous blob).
func TestComputeBBoxBackgroundSubtractionLocalizesObject(t *testing.T) {
	// Frame 10×1: background is all gray (100).
	// Object moves from pixels 1-4 to pixels 5-8 (adjacent, no gap).
	// diff(prev,cur): pixels 1-8 all change → one wide component.
	// diff(bg,cur):   only pixels 5-8 differ from bg → tight right-side bbox.
	w, h := 10, 1
	bg := make([]byte, w*h)
	for i := range bg {
		bg[i] = 100
	}
	prev := make([]byte, w*h)
	copy(prev, bg)
	prev[1], prev[2], prev[3], prev[4] = 200, 200, 200, 200
	cur := make([]byte, w*h)
	copy(cur, bg)
	cur[5], cur[6], cur[7], cur[8] = 200, 200, 200, 200

	// Old approach: diff(prev, cur) — pixels 1-8 all differ, one connected component.
	bboxOld, _ := computeBBox(prev, cur, w, h, nil)
	// New approach: diff(bg, cur) — only pixels 5-8 differ from background.
	bboxNew, _ := computeBBox(bg, cur, w, h, nil)

	// Old bbox spans the merged blob (pixels 1-8).
	if bboxOld.W < 0.7 {
		t.Errorf("old approach: expected wide bbox covering pixels 1-8, got W=%.2f", bboxOld.W)
	}

	// New bbox is only on the right (where object IS now).
	if bboxNew.X < 0.45 {
		t.Errorf("background subtraction: expected bbox starting at pixel 5 (X≥0.5), got X=%.2f", bboxNew.X)
	}
	if bboxNew.W > 0.45 {
		t.Errorf("background subtraction: expected narrow bbox (W≤0.4), got W=%.2f", bboxNew.W)
	}
}

// TestComputeBBoxPicksLargestConnectedBlob is the corridor scenario:
// scattered diff pixels at top and bottom (shadows, illumination) with a dense
// person-shaped blob in the middle. Connected components should select the middle
// blob; the old min/max approach spans the entire frame height.
func TestComputeBBoxPicksLargestConnectedBlob(t *testing.T) {
	w, h := 10, 10
	a := make([]byte, w*h)
	b := make([]byte, w*h)
	// Scattered noise above the person — isolated pixels, not adjacent to blob.
	b[0*w+1] = 200 // (1,0)
	b[1*w+7] = 200 // (7,1)
	// Person blob: dense 3×3 block at rows 4-6, cols 3-5 (9 pixels, totalDiff=1800).
	for y := 4; y <= 6; y++ {
		for x := 3; x <= 5; x++ {
			b[y*w+x] = 200
		}
	}
	// Scattered noise below — isolated pixels, not adjacent to blob.
	b[8*w+2] = 200 // (2,8)
	b[9*w+8] = 200 // (8,9)

	got, found := computeBBox(a, b, w, h, nil)
	if !found {
		t.Fatal("expected found=true for dense blob")
	}
	// Bbox must surround the person blob (rows 4-6, cols 3-5), not the whole frame.
	if got.Y < 0.35 || got.Y > 0.45 {
		t.Errorf("expected Y≈0.40 (person blob rows 4-6), got Y=%.3f — bbox spans wrong region", got.Y)
	}
	if got.H > 0.40 {
		t.Errorf("expected H≈0.30 (3 rows), got H=%.3f — bbox too tall (includes noise rows)", got.H)
	}
	if got.X < 0.25 || got.X > 0.35 {
		t.Errorf("expected X≈0.30 (cols 3-5), got X=%.3f", got.X)
	}
}
