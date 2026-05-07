//go:build windows

package exec

import (
	"io"
	"os/exec"
)

func (c *FFmpegCommander) Start(name string, args ...string) (Process, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd, stdin: stdin}, nil
}

func (p *osProcess) Terminate() error {
	if p.stdin != nil {
		_, _ = p.stdin.Write([]byte("q\n"))
		_ = p.stdin.Close()
	}
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
