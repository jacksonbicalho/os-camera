package recorder

import (
	"io"
	"os/exec"
	"syscall"
)

type FFmpegCommander struct{}

func NewFFmpegCommander() *FFmpegCommander {
	return &FFmpegCommander{}
}

func (c *FFmpegCommander) Start(name string, args ...string) (Process, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd}, nil
}

type osProcess struct {
	cmd *exec.Cmd
}

func (p *osProcess) Terminate() error {
	return p.cmd.Process.Signal(syscall.SIGTERM)
}

func (p *osProcess) Wait() error {
	return p.cmd.Wait()
}
