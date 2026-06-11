package motion

import (
	"bytes"
	"context"
	"io"
	"log/slog"
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

// grayToRGB expands a grayscale frame to RGB24 (R=G=B=v), matching the full-res
// RGB pipe the detector now reads. With equal channels the in-process downscale
// reproduces the original grayscale values, so the diff/bbox tests stay valid.
func grayToRGB(gray []byte) []byte {
	out := make([]byte, len(gray)*3)
	for i, v := range gray {
		out[3*i], out[3*i+1], out[3*i+2] = v, v, v
	}
	return out
}

func TestDetectorRecordsEventWhenDiffExceedsThreshold(t *testing.T) {
	frameSize := 4 // 2×2 px grayscale
	// frame1 = black, frame2 = white → diff=1.0 > threshold=0.05
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...))

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
	frameData := grayToRGB(bytes.Repeat([]byte{128}, frameSize*2))

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
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...))

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
	frameData := grayToRGB(bytes.Repeat([]byte{128}, frameSize*2))

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
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...))

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
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...))

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
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...))

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
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...))

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

func TestDetectorCachesZonesAcrossFrames(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	// 3 frames → 2 diffs → zones consulted twice without caching.
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...))

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1, CooldownSeconds: 0}
	zoneCalls := 0
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		nil, nil, func() []zones.Zone { zoneCalls++; return nil })

	det.processFrames(context.Background())

	if zoneCalls != 1 {
		t.Errorf("expected getZones queried once (cached across frames), got %d", zoneCalls)
	}
}

func TestDetectorReloadZonesRefreshesCache(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize),
		append(bytes.Repeat([]byte{255}, frameSize),
			bytes.Repeat([]byte{0}, frameSize)...)...))

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir, nil)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1, CooldownSeconds: 0}
	zoneCalls := 0
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		nil, nil, func() []zones.Zone { zoneCalls++; return nil })

	det.processFrames(context.Background()) // loads zones once (cached)
	det.reloadZones()                       // forces a refresh

	if zoneCalls != 2 {
		t.Errorf("expected reloadZones to re-query (1 cached load + 1 reload), got %d", zoneCalls)
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
	frameData := grayToRGB(append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...))

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
	frameData := grayToRGB(append(append(frame0, frame1...), frame2...))

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
