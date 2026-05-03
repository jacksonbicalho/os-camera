package streaming_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/streaming"
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

func TestHLSStreamerStartsFFmpegWithCorrectArguments(t *testing.T) {
	tmpDir := t.TempDir()

	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, cmd, discardLogger())
	if err := s.Start(); err != nil {
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
	if !containsSequence(args, "-f", "hls") {
		t.Error("expected -f hls in args")
	}
	if !containsSequence(args, "-hls_time", "2") {
		t.Error("expected -hls_time 2 in args")
	}
	wantPlaylist := filepath.Join(tmpDir, "entrada", "index.m3u8")
	if args[len(args)-1] != wantPlaylist {
		t.Errorf("expected playlist %q, got %q", wantPlaylist, args[len(args)-1])
	}
}

func TestHLSStreamerUsesStreamCopy(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if !containsSequence(args, "-c", "copy") {
		t.Error("expected -c copy in args for stream copy")
	}
	for i, a := range args {
		if a == "-bsf:v" {
			t.Errorf("unexpected -bsf:v %q: RTSP stream is already Annex B", args[i+1])
		}
	}
}

func TestHLSStreamerUsesIndependentSegments(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	for i, a := range args {
		if a == "-hls_flags" && i+1 < len(args) {
			flags := args[i+1]
			if !contains(flags, "independent_segments") {
				t.Errorf("expected independent_segments in -hls_flags, got %q", flags)
			}
			return
		}
	}
	t.Error("expected -hls_flags in args")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHLSStreamerStopFinalizesStream(t *testing.T) {
	tmpDir := t.TempDir()

	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	proc := &trackingProcess{}
	cmd := &fakeCommander{process: proc}

	s := streaming.NewHLSStreamer(camera, server, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s.Stop()

	if !proc.terminated {
		t.Error("expected Terminate() to be called")
	}
	if !proc.waited {
		t.Error("expected Wait() to be called after Terminate()")
	}
}

func TestHLSStreamerCreatesOutputDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	camera := config.CameraConfig{ID: "entrada", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	s := streaming.NewHLSStreamer(camera, server, &fakeCommander{}, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDir := filepath.Join(tmpDir, "entrada")
	if _, err := os.Stat(wantDir); os.IsNotExist(err) {
		t.Errorf("expected directory %q to exist", wantDir)
	}
}
