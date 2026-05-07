package exec

import (
	"io"
	"os/exec"
)

type Process interface {
	Terminate() error
	Wait() error
}

type Commander interface {
	Start(name string, args ...string) (Process, error)
}

type FFmpegCommander struct{}

func NewFFmpegCommander() *FFmpegCommander {
	return &FFmpegCommander{}
}

type osProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

func (p *osProcess) Wait() error {
	return p.cmd.Wait()
}
