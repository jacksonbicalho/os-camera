package motion

import (
	"fmt"
	"io"
	"os/exec"
	"syscall"
)

type ffmpegFrameCommander struct{}

func newFFmpegFrameCommander() *ffmpegFrameCommander {
	return &ffmpegFrameCommander{}
}

func (c *ffmpegFrameCommander) Start(url string, width, height, fps int) (frameProcess, error) {
	vf := fmt.Sprintf("fps=%d,scale=%d:%d,format=gray", fps, width, height)
	args := []string{
		"-rtsp_transport", "tcp",
		"-i", url,
		"-vf", vf,
		"-f", "rawvideo",
		"-pix_fmt", "gray",
		"pipe:1",
	}
	cmd := exec.Command("ffmpeg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = io.Discard

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &ffmpegFrameProcess{cmd: cmd, stdout: stdout}, nil
}

type ffmpegFrameProcess struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
}

func (p *ffmpegFrameProcess) Read(b []byte) (int, error) {
	return p.stdout.Read(b)
}

func (p *ffmpegFrameProcess) Terminate() error {
	if p.cmd.Process != nil {
		syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT)
	}
	return nil
}

func (p *ffmpegFrameProcess) Wait() error {
	return p.cmd.Wait()
}
