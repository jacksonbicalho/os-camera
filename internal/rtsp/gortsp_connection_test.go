package rtsp_test

import (
	"testing"

	"camera/internal/rtsp"
)

func TestGortspConnectionIsNotOpenByDefault(t *testing.T) {
	conn := rtsp.NewGortspConnection()

	if conn.IsOpen() {
		t.Error("expected connection to be closed before Open()")
	}
}

func TestGortspConnectionFailsWithMalformedURL(t *testing.T) {
	conn := rtsp.NewGortspConnection()

	err := conn.Open("://not-a-valid-url")

	if err == nil {
		t.Error("expected error when opening malformed URL")
	}
}

func TestGortspConnectionReadFrameReturnsErrorWhenNotOpen(t *testing.T) {
	conn := rtsp.NewGortspConnection()

	_, err := conn.ReadFrame()

	if err == nil {
		t.Error("expected error when reading frame from closed connection")
	}
}

func TestGortspConnectionFailsToConnectToClosedPort(t *testing.T) {
	conn := rtsp.NewGortspConnection()

	err := conn.Open("rtsp://localhost:1/stream")

	if err == nil {
		t.Error("expected error when connecting to closed port")
	}
	if conn.IsOpen() {
		t.Error("expected connection to remain closed after failed Open()")
	}
}
