package recorder_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/recorder"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeProcess struct{}

func (p *fakeProcess) Terminate() error { return nil }
func (p *fakeProcess) Wait() error      { return nil }

type trackingProcess struct {
	terminated bool
	waited     bool
}

func (p *trackingProcess) Terminate() error { p.terminated = true; return nil }
func (p *trackingProcess) Wait() error      { p.waited = true; return nil }

type fakeCommander struct {
	calls   [][]string
	process exec.Process
}

func (f *fakeCommander) Start(name string, args ...string) (exec.Process, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.process != nil {
		return f.process, nil
	}
	return &fakeProcess{}, nil
}

func containsSequence(haystack []string, key, value string) bool {
	for i := 0; i < len(haystack)-1; i++ {
		if haystack[i] == key && haystack[i+1] == value {
			return true
		}
	}
	return false
}

func TestOutputPatternBuildsFilename(t *testing.T) {
	ts := time.Date(2026, 4, 30, 23, 0, 0, 0, time.UTC)

	got := recorder.OutputPattern("/data", "entrada", ts)

	want := "/data/entrada/2026/04/30/%Y%m%d%H%M%S.mp4"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestOutputDirUsesUTCTime(t *testing.T) {
	ts := time.Date(2026, 4, 30, 23, 0, 0, 0, time.UTC)

	got := recorder.OutputDir("/data", "entrada", ts)

	want := "/data/entrada/2026/04/30"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRecorderCreatesDirectoryBeforeStarting(t *testing.T) {
	tmpDir := t.TempDir()
	ts := time.Date(2026, 4, 30, 23, 0, 0, 0, time.UTC)

	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	storage := config.StorageConfig{Path: tmpDir}
	defaults := config.DefaultsConfig{}

	rec := recorder.NewRecorder(camera, storage, defaults, &fakeCommander{}, discardLogger())
	if err := rec.Start(ts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDir := filepath.Join(tmpDir, "entrada", "2026", "04", "30")
	if _, err := os.Stat(wantDir); os.IsNotExist(err) {
		t.Errorf("expected directory %q to exist", wantDir)
	}
}

func TestRecorderStartsFFmpegWithCorrectArguments(t *testing.T) {
	tmpDir := t.TempDir()
	ts := time.Date(2026, 4, 30, 14, 30, 0, 0, time.UTC)

	camera := config.CameraConfig{
		ID:            "entrada",
		RTSPURL:       "rtsp://192.168.1.10:554/stream",
		ChunkDuration: config.Duration(5 * time.Minute),
	}
	storage := config.StorageConfig{Path: tmpDir}
	defaults := config.DefaultsConfig{}

	cmd := &fakeCommander{}
	rec := recorder.NewRecorder(camera, storage, defaults, cmd, discardLogger())
	if err := rec.Start(ts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cmd.calls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(cmd.calls))
	}
	args := cmd.calls[0]
	if args[0] != "ffmpeg" {
		t.Errorf("expected command %q, got %q", "ffmpeg", args[0])
	}
	if !containsSequence(args, "-i", "rtsp://192.168.1.10:554/stream") {
		t.Error("expected -i <rtsp_url> in args")
	}
	if !containsSequence(args, "-c", "copy") {
		t.Error("expected -c copy in args")
	}
	if !containsSequence(args, "-segment_time", "300") {
		t.Error("expected -segment_time 300 in args")
	}
	if !containsSequence(args, "-avoid_negative_ts", "make_zero") {
		t.Error("expected -avoid_negative_ts make_zero in args")
	}
	if !containsSequence(args, "-strftime", "1") {
		t.Error("expected -strftime 1 in args")
	}
	wantPattern := filepath.Join(tmpDir, "entrada", "2026", "04", "30") + "/%Y%m%d%H%M%S.mp4"
	if args[len(args)-1] != wantPattern {
		t.Errorf("expected output pattern %q, got %q", wantPattern, args[len(args)-1])
	}
}

func TestRecorderStopFinalizesChunk(t *testing.T) {
	tmpDir := t.TempDir()
	ts := time.Date(2026, 4, 30, 14, 30, 0, 0, time.UTC)

	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	storage := config.StorageConfig{Path: tmpDir}
	defaults := config.DefaultsConfig{}

	proc := &trackingProcess{}
	cmd := &fakeCommander{process: proc}

	rec := recorder.NewRecorder(camera, storage, defaults, cmd, discardLogger())
	if err := rec.Start(ts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec.Stop()

	if !proc.terminated {
		t.Error("expected Terminate() to be called")
	}
	if !proc.waited {
		t.Error("expected Wait() to be called after Terminate()")
	}
}
