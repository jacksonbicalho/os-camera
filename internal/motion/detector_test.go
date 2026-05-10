package motion

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
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
	tmpDir := t.TempDir()
	frameSize := 4 // 2×2 px grayscale
	// frame1 = black, frame2 = white → diff=1.0 > threshold=0.05
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	det.processFrames()

	events := readAllEvents(t, tmpDir, "entrada")
	if len(events) != 1 {
		t.Fatalf("expected 1 motion event, got %d", len(events))
	}
}

func TestDetectorIgnoresSmallDiff(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	// Both frames identical → diff=0 < threshold=0.05
	frameData := bytes.Repeat([]byte{128}, frameSize*2)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	det.processFrames()

	events := readAllEvents(t, tmpDir, "entrada")
	if len(events) != 0 {
		t.Fatalf("expected 0 motion events, got %d", len(events))
	}
}

func TestDetectorTimestampIsApproxNow(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil, nil)

	before := time.Now().UTC().Truncate(time.Second)
	det.processFrames()
	after := time.Now().UTC().Add(time.Second)

	events := readAllEvents(t, tmpDir, "entrada")
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	evTime, err := time.Parse(time.RFC3339, events[0]["time"].(string))
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	if evTime.Before(before) || evTime.After(after) {
		t.Errorf("event time %v outside range [%v, %v]", evTime, before, after)
	}
}

func TestDetectorNotifyRawCalledForSubThresholdDiff(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4
	// Both frames identical → diff=0 < threshold=0.05; notifyRaw must still fire
	frameData := bytes.Repeat([]byte{128}, frameSize*2)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	var rawEvents []Event
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, func(ev Event) {
		rawEvents = append(rawEvents, ev)
	}, nil)

	det.processFrames()

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
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	var notified, rawNotified int
	det := newDetector("entrada", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		func(ev Event) { notified++ },
		func(ev Event) { rawNotified++ },
		nil,
	)

	det.processFrames()

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
	st := newStore(tmpDir)

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

	det.processFrames()

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
	st := newStore(tmpDir)

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

	det.processFrames()

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
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1, CooldownSeconds: 0}
	var notified int
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(),
		func(Event) { notified++ }, nil, nil)

	det.processFrames()

	if notified != 2 {
		t.Errorf("expected 2 notifies when cooldown=0, got %d", notified)
	}
}

func TestDetectorExclusionZoneSuppressesEvent(t *testing.T) {
	tmpDir := t.TempDir()
	frameSize := 4 // 2×2 frame
	// frame1=black, frame2=white → full diff; but whole frame is excluded → score=0, no event
	frameData := append(bytes.Repeat([]byte{0}, frameSize), bytes.Repeat([]byte{255}, frameSize)...)

	proc := &fakeFrameProcess{r: bytes.NewReader(frameData)}
	cmd := &fakeFrameCommander{process: proc}
	st := newStore(tmpDir)

	cfg := config.MotionConfig{Enabled: true, Threshold: 0.05, FPS: 1}
	fullZone := zones.Zone{X: 0, Y: 0, W: 1, H: 1}
	det := newDetector("cam", "rtsp://fake", 2, 2, cfg, cmd, st, discardLogger(), nil, nil,
		func() []zones.Zone { return []zones.Zone{fullZone} })

	det.processFrames()

	events := readAllEvents(t, tmpDir, "cam")
	if len(events) != 0 {
		t.Fatalf("expected 0 events with full exclusion zone, got %d", len(events))
	}
}

func readAllEvents(t *testing.T, basePath, cameraID string) []map[string]any {
	t.Helper()
	var events []map[string]any
	root := filepath.Join(basePath, cameraID)
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "motion.ndjson" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var ev map[string]any
			json.Unmarshal(sc.Bytes(), &ev)
			events = append(events, ev)
		}
		return nil
	})
	return events
}
