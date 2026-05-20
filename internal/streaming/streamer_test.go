package streaming_test

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/ffprobe"
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
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
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
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
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
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
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

	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
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

	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, &fakeCommander{}, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDir := filepath.Join(tmpDir, "entrada")
	if _, err := os.Stat(wantDir); os.IsNotExist(err) {
		t.Errorf("expected directory %q to exist", wantDir)
	}
}

func containsArg(haystack []string, s string) bool {
	for _, a := range haystack {
		if a == s {
			return true
		}
	}
	return false
}

func TestHLSStreamerUsesWideSegmentPattern(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPattern := filepath.Join(tmpDir, "cam1", "%06d.ts")
	if !containsArg(cmd.calls[0], wantPattern) {
		t.Errorf("expected segment pattern %q in args, got %v", wantPattern, cmd.calls[0])
	}
}

func TestHLSStreamerDVRDisabledUsesListSizeFiveWithDeleteSegments(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir, HLSDVRSeconds: 0}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if !containsSequence(args, "-hls_list_size", "5") {
		t.Error("expected -hls_list_size 5 when DVR disabled")
	}
	for i, a := range args {
		if a == "-hls_flags" && i+1 < len(args) {
			if !contains(args[i+1], "delete_segments") {
				t.Errorf("expected delete_segments in -hls_flags when DVR disabled, got %q", args[i+1])
			}
			return
		}
	}
	t.Error("expected -hls_flags in args")
}

func TestHLSStreamerDVREnabledUsesCalculatedListSizeAndNoDeleteSegments(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir, HLSDVRSeconds: 1200} // 20 min

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	// 1200s / 2s per segment = 600
	if !containsSequence(args, "-hls_list_size", "600") {
		t.Errorf("expected -hls_list_size 600 for 1200s DVR, got args: %v", args)
	}
	for i, a := range args {
		if a == "-hls_flags" && i+1 < len(args) {
			flags := args[i+1]
			if contains(flags, "delete_segments") {
				t.Errorf("expected no delete_segments in DVR mode, got flags %q", flags)
			}
			if !contains(flags, "program_date_time") {
				t.Errorf("expected program_date_time in DVR mode flags, got %q", flags)
			}
			return
		}
	}
	t.Error("expected -hls_flags in args")
}

func TestHLSStreamerTranscodesNonH264VideoToH264(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{VideoCodec: "hevc", HasAudio: true}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if !containsArg(args, "libx264") {
		t.Error("expected libx264 in args when video codec is hevc")
	}
	if containsSequence(args, "-c", "copy") {
		t.Error("expected no -c copy when transcoding is needed")
	}
	if containsArg(args, "-an") {
		t.Error("expected no -an when camera has audio")
	}
}

func TestHLSStreamerAddsAnFlagWhenNoAudio(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{HasAudio: false}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsArg(cmd.calls[0], "-an") {
		t.Error("expected -an in ffmpeg args when HasAudio = false")
	}
}

func TestHLSStreamerDoesNotAddAnFlagWhenHasAudio(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://192.168.1.10:554/stream"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{HasAudio: true}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if containsArg(cmd.calls[0], "-an") {
		t.Error("expected no -an in ffmpeg args when HasAudio = true")
	}
}

func TestHLSStreamerModeH264AlwaysTranscodes(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream", HLSVideoMode: "h264"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{VideoCodec: "h264", HasAudio: false}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if !containsArg(args, "libx264") {
		t.Error("expected libx264 when mode=h264, even if stream is already h264")
	}
	if containsSequence(args, "-c", "copy") {
		t.Error("expected no -c copy when mode=h264")
	}
}

func TestHLSStreamerModeCopyNeverTranscodes(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream", HLSVideoMode: "copy"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{VideoCodec: "hevc", HasAudio: false}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if containsArg(args, "libx264") {
		t.Error("expected no libx264 when mode=copy, even if stream is hevc")
	}
	if !containsSequence(args, "-c", "copy") {
		t.Error("expected -c copy when mode=copy")
	}
}

func TestHLSStreamerModeAutoTranscodesNonH264(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream", HLSVideoMode: "auto"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{VideoCodec: "hevc", HasAudio: false}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsArg(cmd.calls[0], "libx264") {
		t.Error("expected libx264 when mode=auto and codec is hevc")
	}
}

func TestHLSStreamerModeAutoUsesStreamCopyForH264(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream", HLSVideoMode: "auto"}
	server := config.ServerConfig{SegmentsPath: tmpDir}
	stream := ffprobe.StreamInfo{VideoCodec: "h264", HasAudio: false}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, stream, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if containsArg(cmd.calls[0], "libx264") {
		t.Error("expected no libx264 when mode=auto and codec is already h264")
	}
	if !containsSequence(cmd.calls[0], "-c", "copy") {
		t.Error("expected -c copy when mode=auto and codec is h264")
	}
}

func intPtr(v int) *int { return &v }

func TestHLSStreamerCustomSegmentSeconds(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream", HLSSegmentSeconds: intPtr(1)}
	server := config.ServerConfig{SegmentsPath: tmpDir}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if !containsSequence(args, "-hls_time", "1") {
		t.Errorf("expected -hls_time 1, got args: %v", args)
	}
}

func TestHLSStreamerCustomListSize(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream", HLSListSize: intPtr(3)}
	server := config.ServerConfig{SegmentsPath: tmpDir, HLSDVRSeconds: 0}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSequence(cmd.calls[0], "-hls_list_size", "3") {
		t.Errorf("expected -hls_list_size 3, got args: %v", cmd.calls[0])
	}
}

func TestHLSStreamerNilFieldsUseDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	camera := config.CameraConfig{ID: "cam1", RTSPURL: "rtsp://host/stream"} // nil *int fields
	server := config.ServerConfig{SegmentsPath: tmpDir}

	cmd := &fakeCommander{}
	s := streaming.NewHLSStreamer(camera, server, ffprobe.StreamInfo{}, cmd, discardLogger())
	if err := s.Start(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := cmd.calls[0]
	if !containsSequence(args, "-hls_time", "2") {
		t.Errorf("expected default -hls_time 2, got args: %v", args)
	}
	if !containsSequence(args, "-hls_list_size", "5") {
		t.Errorf("expected default -hls_list_size 5, got args: %v", args)
	}
}
