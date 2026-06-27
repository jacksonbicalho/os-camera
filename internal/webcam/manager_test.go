package webcam

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestPublishArgs(t *testing.T) {
	args := publishArgs(Device{Path: "/dev/video0", RTSPName: "webcam0"}, "127.0.0.1:8554", 30, "1280x720")
	joined := strings.Join(args, " ")
	for _, want := range []string{"-f v4l2", "-i /dev/video0", "-c:v libx264", "-video_size 1280x720", "-framerate 30", "-force_key_frames", "-g 30"} {
		if !strings.Contains(joined, want) {
			t.Errorf("publishArgs faltando %q em: %s", want, joined)
		}
	}
	if args[len(args)-1] != "rtsp://127.0.0.1:8554/webcam0" {
		t.Errorf("último arg deve ser a URL RTSP, got %q", args[len(args)-1])
	}
}

func TestWebcamName(t *testing.T) {
	m := New(context.Background(), "127.0.0.1:8554", discard())
	cases := []struct {
		url      string
		wantName string
		wantOK   bool
	}{
		{"rtsp://127.0.0.1:8554/webcam0", "webcam0", true},
		{"rtsp://127.0.0.1:8554/webcam2", "webcam2", true},
		{"rtsp://192.168.1.10:554/stream", "", false}, // câmera de rede
		{"rtsp://127.0.0.1:8554/outro", "", false},     // path não-webcam
		{"rtsp://127.0.0.1:8554/", "", false},
	}
	for _, c := range cases {
		name, ok := m.WebcamName(c.url)
		if name != c.wantName || ok != c.wantOK {
			t.Errorf("WebcamName(%q) = (%q,%v), quer (%q,%v)", c.url, name, ok, c.wantName, c.wantOK)
		}
	}
}

func TestEnsure_DeviceDesconhecido_NaoQuebra(t *testing.T) {
	m := New(context.Background(), "127.0.0.1:0", discard())
	m.Ensure("webcam999") // device inexistente → só loga, não panica
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pubs) != 0 {
		t.Fatalf("não deveria registrar publisher para device inexistente")
	}
}

func TestTailWriter(t *testing.T) {
	var w tailWriter
	w.Write([]byte("erro de abertura: "))
	w.Write([]byte("Cannot open /dev/video0\n"))
	if got := w.String(); !strings.Contains(got, "Cannot open /dev/video0") {
		t.Fatalf("tail inesperado: %q", got)
	}
}
