//go:build !windows

package motion

import (
	"io"
	"os/exec"
	"syscall"
)

func (c *ffmpegFrameCommander) Start(url string, width, height, fps int) (frameProcess, error) {
	cmd := exec.Command("ffmpeg", ffmpegArgs(url, width, height, fps)...)
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

func (p *ffmpegFrameProcess) Terminate() error {
	if p.cmd.Process != nil {
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT)
	}
	return nil
}
