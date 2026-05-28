package motion

import (
	"context"

	"camera/internal/exec"
)

type ffmpegSnapshotGrabber struct {
	commander exec.Commander
}

func newFFmpegSnapshotGrabber(cmd exec.Commander) *ffmpegSnapshotGrabber {
	return &ffmpegSnapshotGrabber{commander: cmd}
}

func (g *ffmpegSnapshotGrabber) Grab(ctx context.Context, rtspURL, destPath string) error {
	proc, err := g.commander.Start("ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-frames:v", "1",
		"-q:v", "2",
		"-y",
		destPath,
	)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- proc.Wait() }()
	select {
	case <-ctx.Done():
		proc.Terminate()
		return ctx.Err()
	case err := <-done:
		return err
	}
}
