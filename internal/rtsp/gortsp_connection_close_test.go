package rtsp

import (
	"testing"
	"time"
)

func TestGortspConnectionReadFrameUnblocksOnClose(t *testing.T) {
	conn := &GortspConnection{
		frames: make(chan Frame, 32),
		done:   make(chan struct{}),
		open:   true,
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := conn.ReadFrame()
		errCh <- err
	}()

	conn.Close()

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected error after Close()")
		}
	case <-time.After(time.Second):
		t.Error("ReadFrame() did not unblock after Close()")
	}
}
