package rtsp_test

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"camera/internal/rtsp"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- connect ---

func TestClientConnectsToRTSPURL(t *testing.T) {
	conn := rtsp.NewFakeConnection(true, nil)
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())

	if err := client.Connect(); err != nil {
		t.Fatalf("expected successful connection, got: %v", err)
	}
	if !client.IsConnected() {
		t.Error("expected client to be in connected state")
	}
}

func TestClientIsNotConnectedBeforeConnect(t *testing.T) {
	conn := rtsp.NewFakeConnection(true, nil)
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())

	if client.IsConnected() {
		t.Error("client must not be connected before calling Connect()")
	}
}

// --- get frame ---

func TestClientGetsFrameAfterConnect(t *testing.T) {
	expectedData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	conn := rtsp.NewFakeConnection(true, expectedData)
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())

	_ = client.Connect()

	frame, err := client.GetFrame()

	if err != nil {
		t.Fatalf("expected frame without error, got: %v", err)
	}
	if len(frame.Data) == 0 {
		t.Error("expected frame with data, got empty frame")
	}
}

func TestClientCannotGetFrameWithoutConnecting(t *testing.T) {
	conn := rtsp.NewFakeConnection(true, nil)
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())

	_, err := client.GetFrame()

	if err == nil {
		t.Error("expected error when getting frame without connecting")
	}
}

// --- close ---

func TestClientClosesConnection(t *testing.T) {
	conn := rtsp.NewFakeConnection(true, nil)
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())
	_ = client.Connect()

	client.Close()

	if client.IsConnected() {
		t.Error("expected client to be disconnected after Close()")
	}
}

func TestClientCannotGetFrameAfterClose(t *testing.T) {
	conn := rtsp.NewFakeConnection(true, []byte{0xFF})
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())
	_ = client.Connect()
	client.Close()

	_, err := client.GetFrame()

	if err == nil {
		t.Error("expected error when getting frame after close")
	}
}

// --- invalid frame ---

func TestClientPropagatesInvalidFrameError(t *testing.T) {
	conn := rtsp.NewFakeConnection(true, nil)
	conn.SetReadError(errors.New("corrupted frame"))
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())
	_ = client.Connect()

	_, err := client.GetFrame()

	if err == nil {
		t.Error("expected error when reading invalid frame")
	}
}

// --- reconnect ---

func TestClientReconnectsAfterFailure(t *testing.T) {
	conn := rtsp.NewFakeConnection(false, nil)
	client := rtsp.NewClient("rtsp://localhost:8554/test", conn, discardLogger())

	_ = client.Connect()
	if client.IsConnected() {
		t.Fatal("client must not be connected after failure")
	}

	conn.SetShouldOpen(true)
	if err := client.Connect(); err != nil {
		t.Fatalf("expected successful reconnection, got: %v", err)
	}
	if !client.IsConnected() {
		t.Error("expected client to be connected after reconnection")
	}
}
