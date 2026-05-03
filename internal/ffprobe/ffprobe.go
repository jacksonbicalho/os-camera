package ffprobe

import (
	"context"
	"os/exec"
)

type Executor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Prober struct {
	exec Executor
}

func NewProber(exec Executor) *Prober {
	return &Prober{exec: exec}
}

func (p *Prober) Probe(ctx context.Context, url string) ([]byte, error) {
	return p.exec.Execute(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		url,
	)
}

type OSExecutor struct{}

func (e *OSExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
