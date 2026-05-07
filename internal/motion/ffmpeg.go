package motion

import (
	"fmt"
	"io"
	"os/exec"
)

type ffmpegFrameCommander struct{}

func newFFmpegFrameCommander() *ffmpegFrameCommander {
	return &ffmpegFrameCommander{}
}

type ffmpegFrameProcess struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

func (p *ffmpegFrameProcess) Read(b []byte) (int, error) {
	return p.stdout.Read(b)
}

func (p *ffmpegFrameProcess) Wait() error {
	return p.cmd.Wait()
}

func ffmpegArgs(url string, width, height, fps int) []string {
	vf := fmt.Sprintf("fps=%d,scale=%d:%d,format=gray", fps, width, height)
	return []string{
		"-rtsp_transport", "tcp",
		"-i", url,
		"-vf", vf,
		"-f", "rawvideo",
		"-pix_fmt", "gray",
		"pipe:1",
	}
}
