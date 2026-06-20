package motion

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"camera/internal/config"
)

// --- downscaleRGBToGray: full-res RGB frame → diff-resolution grayscale ---

// TestDownscaleRGBToGrayNoScale verifies that with equal source/destination
// dimensions the function only collapses RGB to gray (equal channels → value).
func TestDownscaleRGBToGrayNoScale(t *testing.T) {
	// 2×1 RGB, equal channels → gray equals the channel value.
	rgb := []byte{200, 200, 200, 100, 100, 100}
	gray := downscaleRGBToGray(rgb, 2, 1, 2, 1)
	want := []byte{200, 100}
	if !bytes.Equal(gray, want) {
		t.Fatalf("no-scale RGB→gray = %v, want %v", gray, want)
	}
}

// TestDownscaleRGBToGrayAverages verifies block-average downscale: a 2×1 RGB
// frame collapsed to 1×1 averages the two pixels.
func TestDownscaleRGBToGrayAverages(t *testing.T) {
	rgb := []byte{200, 200, 200, 100, 100, 100} // 2×1
	gray := downscaleRGBToGray(rgb, 2, 1, 1, 1)
	if len(gray) != 1 {
		t.Fatalf("expected 1 px, got %d", len(gray))
	}
	if gray[0] < 149 || gray[0] > 151 {
		t.Fatalf("downscaled gray = %d, want ~150 (avg of 200,100)", gray[0])
	}
}

// --- annotateRGBFrame: draw box + score onto a full-res RGB frame ---

// TestAnnotateRGBFrameFullRes verifies the annotated snapshot is a valid JPEG at
// full resolution (not the downscaled diff resolution) and preserves color.
func TestAnnotateRGBFrameFullRes(t *testing.T) {
	w, h := 16, 8
	rgb := make([]byte, w*h*3)
	for i := 0; i < w*h; i++ {
		rgb[3*i] = 200  // R
		rgb[3*i+1] = 40 // G
		rgb[3*i+2] = 40 // B  → reddish, clearly not gray
	}
	out := annotateRGBFrame(rgb, w, h, BBox{0.25, 0.25, 0.5, 0.5}, 0.42, ColorGlobal, true)

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("annotateRGBFrame produced invalid JPEG: %v", err)
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("snapshot dims = %dx%d, want %dx%d (full-res)", img.Bounds().Dx(), img.Bounds().Dy(), w, h)
	}
	// Color must survive: a background pixel stays reddish (R clearly > B).
	r, _, b, _ := img.At(1, 1).RGBA()
	if r>>8 <= b>>8 {
		t.Errorf("expected reddish pixel (R>B), got R=%d B=%d — color not preserved", r>>8, b>>8)
	}
}

// --- integration: snapshot is the detection frame, full-res, color, no grab ---

func solidRGB(w, h int, r, g, b byte) []byte {
	out := make([]byte, w*h*3)
	for i := 0; i < w*h; i++ {
		out[3*i], out[3*i+1], out[3*i+2] = r, g, b
	}
	return out
}

// leftHalfYellowRGB builds a w×h RGB frame whose left half is bright yellow
// (240,240,40) and right half neutral gray (100,100,100).
func leftHalfYellowRGB(w, h int) []byte {
	out := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			if x < w/2 {
				out[i], out[i+1], out[i+2] = 240, 240, 40
			} else {
				out[i], out[i+1], out[i+2] = 100, 100, 100
			}
		}
	}
	return out
}

// TestDetectorSnapshotIsDetectionFrameFullRes is the core contract of the fix:
// the saved snapshot must be the very frame that triggered the event (full-res,
// color), with the bbox computed from that same frame — NOT a late RTSP grab.
// No grabber is set; the detector must annotate and save the detection frame.
func TestDetectorSnapshotIsDetectionFrameFullRes(t *testing.T) {
	w, h := 16, 4
	bg := solidRGB(w, h, 100, 100, 100) // static background
	detFrame := leftHalfYellowRGB(w, h) // object enters on the LEFT half
	frameData := append(append([]byte{}, bg...), detFrame...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	tmp := t.TempDir()
	type rec struct {
		frame string
		bbox  BBox
		ts    time.Time
	}
	var mu sync.Mutex
	var recs []rec
	st := newStore(tmp, func(_ string, ts time.Time, _ float64, frame, _, _ string, bbox BBox) {
		mu.Lock()
		recs = append(recs, rec{frame: frame, bbox: bbox, ts: ts})
		mu.Unlock()
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	// width/height are the full-res pipe dimensions; no grabber is injected.
	d := newDetector("cam", "rtsp://fake", w, h, cfg, cmd, st, discardLogger(), nil, nil, nil)

	d.processFrames(context.Background())

	// The snapshot is annotated and saved synchronously from the detection frame,
	// so the event is already recorded once processFrames returns.
	mu.Lock()
	got := append([]rec{}, recs...)
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("expected a motion event recorded")
	}
	r0 := got[len(got)-1]

	// bbox must reflect the detection frame: object on the LEFT → X < 0.5.
	if r0.bbox.X >= 0.5 {
		t.Errorf("bbox X=%.2f — snapshot/bbox not taken from the detection frame (object was on the left)", r0.bbox.X)
	}

	// The saved snapshot must be a full-res (w×h) color JPEG of the detection frame.
	path := filepath.Join(tmp, "cam", r0.ts.UTC().Format("2006/01/02"), r0.frame)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot %q: %v", path, err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("snapshot dims = %dx%d, want %dx%d (full-res detection frame)", img.Bounds().Dx(), img.Bounds().Dy(), w, h)
	}
	// Left half was yellow (R,G high, B low) → color must be preserved.
	r, _, b, _ := img.At(w/4, h/2).RGBA()
	if r>>8 < 150 || b>>8 > 120 {
		t.Errorf("left-half pixel R=%d B=%d — expected yellow (R high, B low); color not preserved", r>>8, b>>8)
	}

	// Junto do _motion.jpg anotado, um _frame.jpg LIMPO do mesmo instante (full-res),
	// usado pelo picker do carrossel.
	cleanName := strings.Replace(r0.frame, "_motion.jpg", "_frame.jpg", 1)
	cleanPath := filepath.Join(tmp, "cam", r0.ts.UTC().Format("2006/01/02"), cleanName)
	cleanData, err := os.ReadFile(cleanPath)
	if err != nil {
		t.Fatalf("clean _frame.jpg não gravado: %v", err)
	}
	cleanImg, err := jpeg.Decode(bytes.NewReader(cleanData))
	if err != nil {
		t.Fatalf("decode clean frame: %v", err)
	}
	if cleanImg.Bounds().Dx() != w || cleanImg.Bounds().Dy() != h {
		t.Fatalf("clean frame dims = %dx%d, want %dx%d", cleanImg.Bounds().Dx(), cleanImg.Bounds().Dy(), w, h)
	}
}
