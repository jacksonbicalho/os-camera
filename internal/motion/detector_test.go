package motion

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/zones"
)

type fakeFrameProcess struct {
	r          io.Reader
	terminated bool
}

func (p *fakeFrameProcess) Read(b []byte) (int, error) { return p.r.Read(b) }
func (p *fakeFrameProcess) Terminate() error           { p.terminated = true; return nil }
func (p *fakeFrameProcess) Wait() error                { return nil }

type fakeFrameCommander struct {
	process *fakeFrameProcess
	started int
}

func (c *fakeFrameCommander) Start(url string, width, height, fps int) (frameProcess, error) {
	c.started++
	return c.process, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDetectorRecordsEventWhenDiffExceedsThreshold(t *testing.T) {
	frameSize := 4 // 2×2 px grayscale
	// frame1 = black, frame2 = white → diff=1.0 > threshold=0.05
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	var captured []Event
	st := newStore(t.TempDir(), func(cameraID string, ts time.Time, score float64, frame, label, color string, bbox BBox) {
		captured = append(captured, Event{Score: score})
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	det.processFrames(context.Background())

	if len(captured) != 1 {
		t.Fatalf("expected 1 motion event, got %d", len(captured))
	}
}

func TestDetectorIgnoresSmallDiff(t *testing.T) {
	frameSize := 4
	// Both frames identical → diff=0 < threshold=0.05
	frameData := bytes.Repeat([]byte{128}, frameSize*2)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	var captured []Event
	st := newStore(t.TempDir(), func(_ string, _ time.Time, score float64, _, _, _ string, _ BBox) {
		captured = append(captured, Event{Score: score})
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	det.processFrames(context.Background())

	if len(captured) != 0 {
		t.Fatalf("expected 0 motion events, got %d", len(captured))
	}
}

func TestDetectorTimestampIsApproxNow(t *testing.T) {
	frameSize := 4
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	var capturedAt time.Time
	st := newStore(t.TempDir(), func(_ string, ts time.Time, _ float64, _, _, _ string, _ BBox) {
		capturedAt = ts
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	before := time.Now().UTC().Truncate(time.Second)
	det.processFrames(context.Background())
	after := time.Now().UTC().Add(time.Second)

	if capturedAt.IsZero() {
		t.Fatal("expected 1 event")
	}
	if capturedAt.Before(before) || capturedAt.After(after) {
		t.Errorf("event time %v outside range [%v, %v]", capturedAt, before, after)
	}
}

func TestDetectorNotifyRawCalledForSubThresholdDiff(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	// Both frames identical → diff=0 < threshold=0.05; notifyRaw must still fire
	frameData := bytes.Repeat([]byte{128}, frameSize*2)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	var rawEvents []Event
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, func(ev Event) {
		rawEvents = append(rawEvents, ev)
	}, nil)

	det.processFrames(context.Background())

	if len(rawEvents) != 1 {
		t.Fatalf("expected 1 raw event, got %d", len(rawEvents))
	}
}

func TestDetectorNotifyRawCalledAlongsideNotify(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	// frame1=black, frame2=white → diff=1.0 > threshold; both notify and notifyRaw must fire
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	var notified, rawNotified int
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		func(ev Event) { notified++ },
		func(ev Event) { rawNotified++ },
		nil,
	)

	det.processFrames(context.Background())

	if notified != 1 {
		t.Fatalf("expected notify called 1 time, got %d", notified)
	}
	if rawNotified != 1 {
		t.Fatalf("expected notifyRaw called 1 time, got %d", rawNotified)
	}
}

func TestDetectorCooldownSuppressesEventsWithinWindow(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	// 3 frames: black → white → black → two diffs of 1.0, both above threshold
	frameData := append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1, CooldownSeconds: 30}
	var notified int
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		func(Event) { notified++ }, nil, nil)

	t0 := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	call := 0
	det.now = func() time.Time {
		call++
		if call == 1 {
			return t0 // first diff: event fires
		}
		return t0.Add(10 * time.Second) // second diff: within 30s cooldown
	}

	det.processFrames(context.Background())

	if notified != 1 {
		t.Errorf("expected 1 notify within cooldown, got %d", notified)
	}
}

func TestDetectorCooldownAllowsEventAfterWindow(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	frameData := append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1, CooldownSeconds: 30}
	var notified int
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		func(Event) { notified++ }, nil, nil)

	t0 := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	call := 0
	det.now = func() time.Time {
		call++
		if call == 1 {
			return t0
		}
		return t0.Add(31 * time.Second) // after cooldown window
	}

	det.processFrames(context.Background())

	if notified != 2 {
		t.Errorf("expected 2 notifies after cooldown expires, got %d", notified)
	}
}

func TestDetectorCooldownZeroDisablesSuppression(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	frameData := append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1, CooldownSeconds: 0}
	var notified int
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		func(Event) { notified++ }, nil, nil)

	det.processFrames(context.Background())

	if notified != 2 {
		t.Errorf("expected 2 notifies when cooldown=0, got %d", notified)
	}
}

func TestDetectorContextCancellationTerminatesProcess(t *testing.T) {
	// fakeBlockProcess blocks until Terminate() is called, simulating an infinite
	// RTSP stream that never returns EOF.
	done := make(chan struct{})
	proc := &fakeBlockProcess{done: done}
	cmd := &fakeBlockCommander{process: proc}
	st := newStore(t.TempDir(), nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		det.processFrames(ctx)
		close(finished)
	}()

	cancel()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("processFrames did not return after context cancellation")
	}
	if !proc.terminated {
		t.Error("expected Terminate() to be called on context cancellation")
	}
}

// fakeBlockProcess blocks Read until Terminate is called.
type fakeBlockProcess struct {
	terminated bool
	once       sync.Once
	done       chan struct{}
}

func (p *fakeBlockProcess) Read(b []byte) (int, error) {
	<-p.done
	return 0, io.EOF
}
func (p *fakeBlockProcess) Terminate() error {
	p.once.Do(func() {
		p.terminated = true
		close(p.done)
	})
	return nil
}
func (p *fakeBlockProcess) Wait() error { return nil }

type fakeBlockCommander struct {
	process *fakeBlockProcess
}

func (c *fakeBlockCommander) Start(url string, width, height, fps int) (frameProcess, error) {
	return c.process, nil
}

func TestDetectorExclusionZoneSuppressesEvent(t *testing.T) {
	frameSize := 4 // 2×2 frame
	// frame1=black, frame2=white → full diff; but whole frame is excluded → score=0, no event
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	var captured []Event
	st := newStore(t.TempDir(), func(_ string, _ time.Time, score float64, _, _, _ string, _ BBox) {
		captured = append(captured, Event{Score: score})
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	fullZone := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil,
		func() []zones.Zone { return []zones.Zone{fullZone} })

	det.processFrames(context.Background())

	if len(captured) != 0 {
		t.Fatalf("expected 0 events with full exclusion zone, got %d", len(captured))
	}
}

// fakeGrabberWithJPEG writes a real JPEG to destPath so recordWithHighRes can
// decode and re-compute the bbox from the grabbed frame content.
type fakeGrabberWithJPEG struct {
	mu    sync.Mutex
	calls int
	frame []byte
	w, h  int
	err   error
}

func (g *fakeGrabberWithJPEG) Grab(_ context.Context, _, destPath string) error {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	if g.err != nil {
		return g.err
	}
	img := image.NewGray(image.Rect(0, 0, g.w, g.h))
	copy(img.Pix, g.frame)
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
}

// TestDetectorHighResBboxRefreshesFromGrabbedFrame verifies that when a
// high-res JPEG is grabbed, the bbox is re-computed from the grabbed frame
// (not reused from the stale detection-frame bbox).
func TestDetectorHighResBboxRefreshesFromGrabbedFrame(t *testing.T) {
	// 4×1 frame: bg=[100,100,100,100], detection frame has object on LEFT (pixel 0).
	w, h := 4, 1
	frameSize := w * h
	bgFrame := bytes.Repeat([]byte{100}, frameSize)
	detectionFrame := []byte{200, 100, 100, 100} // object at pixel 0 (left)
	frameData := append(bgFrame, detectionFrame...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	// Grabbed JPEG has object at pixel 3 (right side).
	grabbedFrame := []byte{100, 100, 100, 200}
	grabber := &fakeGrabberWithJPEG{frame: grabbedFrame, w: w, h: h}

	var mu sync.Mutex
	var capturedBBoxes []BBox
	st := newStore(t.TempDir(), func(_ string, _ time.Time, _ float64, _, _, _ string, bbox BBox) {
		mu.Lock()
		capturedBBoxes = append(capturedBBoxes, bbox)
		mu.Unlock()
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("cam", "rtsp://fake", w, h, cfg, cmd, st, discardLogger(), nil, nil, nil)
	det.grabber = grabber

	det.processFrames(context.Background())

	// Wait for async recordWithHighRes to finish.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(capturedBBoxes)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	bboxes := capturedBBoxes
	mu.Unlock()

	if len(bboxes) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(bboxes))
	}

	// Grabbed frame had object on right (pixel 3/4 → X≥0.5).
	// If bbox were stale from detection frame it would have X<0.5 (object was on left).
	bbox := bboxes[0]
	if bbox.X < 0.5 {
		t.Errorf("bbox should reflect grabbed frame (object on right, X≥0.5), got X=%.2f — stale detection bbox was used", bbox.X)
	}
}

// --- snapshot grabber ---

type fakeSnapshotGrabber struct {
	mu      sync.Mutex
	calls   []snapshotGrabCall
	err     error
}

type snapshotGrabCall struct {
	rtspURL  string
	destPath string
}

func (g *fakeSnapshotGrabber) Grab(_ context.Context, rtspURL, destPath string) error {
	g.mu.Lock()
	g.calls = append(g.calls, snapshotGrabCall{rtspURL: rtspURL, destPath: destPath})
	g.mu.Unlock()
	return g.err
}

func TestDetectorGrabsHighResSnapshotOnMotionEvent(t *testing.T) {
	frameSize := 4 // 2×2 px
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	grabber := &fakeSnapshotGrabber{}

	var mu sync.Mutex
	var recorded []string
	st := newStore(t.TempDir(), func(_ string, _ time.Time, _ float64, frame, _, _ string, _ BBox) {
		mu.Lock()
		recorded = append(recorded, frame)
		mu.Unlock()
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("cam1", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)
	det.grabber = grabber

	det.processFrames(context.Background())

	// Wait for async grab + record to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		grabber.mu.Lock()
		n := len(grabber.calls)
		grabber.mu.Unlock()
		mu.Lock()
		r := len(recorded)
		mu.Unlock()
		if n > 0 && r > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	grabber.mu.Lock()
	calls := grabber.calls
	grabber.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("expected 1 grab call, got %d", len(calls))
	}
	if calls[0].rtspURL != "rtsp://fake" {
		t.Errorf("grab called with wrong URL: %q", calls[0].rtspURL)
	}

	// record must happen AFTER grab (event only reaches frontend with high-res ready).
	mu.Lock()
	n := len(recorded)
	mu.Unlock()
	if n != 1 {
		t.Errorf("expected 1 recorded event after grab, got %d", n)
	}
}

// TestDetectorBBoxUsesBackgroundSubtraction verifies that the bbox for a motion
// event reflects where the object IS in the current frame (vs the background),
// not where it moved from (departure region in prev).
func TestDetectorBBoxUsesBackgroundSubtraction(t *testing.T) {
	// Frame 4×1 (w=4, h=1).
	// Frame 0 [init] : [100,100,100,100] — static background, no object
	// Frame 1 [left] : [200,100,100,100] — object enters at pixel 0 (left)
	// Frame 2 [right]: [100,100,100,200] — object moves to pixel 3 (right)
	frame0 := []byte{100, 100, 100, 100}
	frame1 := []byte{200, 100, 100, 100}
	frame2 := []byte{100, 100, 100, 200}
	frameData := append(append(frame0, frame1...), frame2...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}

	var capturedBBoxes []BBox
	st := newStore(t.TempDir(), func(_ string, _ time.Time, _ float64, _, _, _ string, bbox BBox) {
		capturedBBoxes = append(capturedBBoxes, bbox)
	})

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("cam", "rtsp://fake", 4, 1, cfg, cmd, st, discardLogger(), nil, nil, nil)
	det.processFrames(context.Background())

	// Both frame1 and frame2 trigger motion (diff > 0.05 from their prev).
	if len(capturedBBoxes) < 2 {
		t.Fatalf("expected ≥2 motion events, got %d", len(capturedBBoxes))
	}

	// For frame2 (object at pixel 3): with background subtraction,
	// bbox should cover only the right side (X≥0.5, W≤0.5).
	// Without background subtraction (using prev): bbox would span pixels 0-3 (full width).
	last := capturedBBoxes[len(capturedBBoxes)-1]
	if last.X < 0.5 {
		t.Errorf("bbox should be on right side (X>=0.5), got X=%.2f — background subtraction not working", last.X)
	}
	if last.W > 0.5 {
		t.Errorf("bbox should be narrow (W<=0.5), got W=%.2f — background subtraction not working", last.W)
	}
}
