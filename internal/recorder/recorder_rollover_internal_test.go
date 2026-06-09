package recorder

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/ffprobe"
)

// blockingProc blocks Wait() until Terminate() is called, simulating an ffmpeg
// process that keeps running until the recorder explicitly stops it.
type blockingProc struct {
	done sync.Once
	ch   chan struct{}
}

func newBlockingProc() *blockingProc { return &blockingProc{ch: make(chan struct{})} }
func (p *blockingProc) Terminate() error {
	p.done.Do(func() { close(p.ch) })
	return nil
}
func (p *blockingProc) Wait() error { <-p.ch; return nil }

type rolloverCommander struct {
	mu      sync.Mutex
	count   int
	started chan int
}

func (c *rolloverCommander) Start(name string, args ...string) (exec.Process, error) {
	c.mu.Lock()
	c.count++
	n := c.count
	c.mu.Unlock()
	c.started <- n
	return newBlockingProc(), nil
}

// TestRecorderRunRollsDirectoryAtDayBoundary verifies that an ffmpeg session
// started just before UTC midnight is restarted once the day rolls over, so
// chunks for the new day land in the new day's directory instead of piling up
// in the directory that was fixed when the process first started.
func TestRecorderRunRollsDirectoryAtDayBoundary(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	storage := config.StorageConfig{Path: tmpDir}

	// Clock: first Start sits 100ms before midnight; subsequent Starts are on
	// the next day. DurationUntilNextDay(first) ~= 100ms so the day timer fires
	// quickly and triggers a real restart.
	day1 := time.Date(2026, 4, 30, 23, 59, 59, int(900*time.Millisecond), time.UTC)
	day2 := time.Date(2026, 5, 1, 0, 0, 1, 0, time.UTC)
	var mu sync.Mutex
	calls := 0
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return day1
		}
		return day2
	}

	cmd := &rolloverCommander{started: make(chan int, 8)}
	rec := NewRecorder(camera, storage, ffprobe.StreamInfo{}, cmd, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec.now = clock

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		rec.Run(ctx, time.Hour) // large reconnect so only the day timer can restart
	}()

	// Wait for the first start, then for the rollover-triggered second start.
	waitStart := func(want int) {
		for {
			select {
			case n := <-cmd.started:
				if n >= want {
					return
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for ffmpeg start #%d", want)
			}
		}
	}
	waitStart(1)
	waitStart(2)

	cancel()
	<-done

	wantDir := filepath.Join(tmpDir, "cam1", "2026", "05", "01")
	if _, err := os.Stat(wantDir); err != nil {
		t.Errorf("expected new day directory %q to be created after rollover: %v", wantDir, err)
	}
}
