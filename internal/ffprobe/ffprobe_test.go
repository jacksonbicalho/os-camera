package ffprobe_test

import (
	"context"
	"errors"
	"testing"

	"camera/internal/ffprobe"
)

type fakeExecutor struct {
	capturedName string
	capturedArgs []string
	output       []byte
	err          error
}

func (f *fakeExecutor) Execute(_ context.Context, name string, args ...string) ([]byte, error) {
	f.capturedName = name
	f.capturedArgs = args
	return f.output, f.err
}

func TestProberCallsFFprobeWithCorrectArguments(t *testing.T) {
	exec := &fakeExecutor{output: []byte(`{}`)}
	p := ffprobe.NewProber(exec)

	if _, err := p.Probe(context.Background(), "rtsp://192.168.1.10:554/stream"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exec.capturedName != "ffprobe" {
		t.Errorf("expected command %q, got %q", "ffprobe", exec.capturedName)
	}
	if !containsSeq(exec.capturedArgs, "-v", "quiet") {
		t.Error("expected -v quiet in args")
	}
	if !containsSeq(exec.capturedArgs, "-print_format", "json") {
		t.Error("expected -print_format json in args")
	}
	if !containsArg(exec.capturedArgs, "-show_streams") {
		t.Error("expected -show_streams in args")
	}
	if !containsArg(exec.capturedArgs, "-show_format") {
		t.Error("expected -show_format in args")
	}
	if exec.capturedArgs[len(exec.capturedArgs)-1] != "rtsp://192.168.1.10:554/stream" {
		t.Errorf("expected URL as last arg, got %q", exec.capturedArgs[len(exec.capturedArgs)-1])
	}
}

func TestProberReturnsFFprobeOutput(t *testing.T) {
	want := []byte(`{"streams":[],"format":{}}`)
	exec := &fakeExecutor{output: want}
	p := ffprobe.NewProber(exec)

	got, err := p.Probe(context.Background(), "rtsp://any")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("expected output %q, got %q", want, got)
	}
}

func TestProberPropagatesExecutorError(t *testing.T) {
	want := errors.New("ffprobe not found")
	exec := &fakeExecutor{err: want}
	p := ffprobe.NewProber(exec)

	_, err := p.Probe(context.Background(), "rtsp://any")
	if !errors.Is(err, want) {
		t.Errorf("expected error %v, got %v", want, err)
	}
}

func containsSeq(args []string, key, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}

func containsArg(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}
