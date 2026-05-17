package ffprobe_test

import (
	"context"
	"log/slog"
	"testing"

	"camera/internal/ffprobe"
)

func TestResolve_ProbesWhenDimensionsUnknown(t *testing.T) {
	// Simula: câmera tem has_audio=true salvo de uma sonda anterior que falhou,
	// mas as dimensões (Width=0, Height=0) ainda são desconhecidas.
	// Resolve deve sondar novamente para detectar as dimensões.
	hasAudio := true
	exec := &fakeExecutor{
		output: []byte(`{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080}]}`),
	}
	prober := ffprobe.NewProber(exec)

	r := ffprobe.Resolver{
		RTSPURL:  "rtsp://cam",
		HasAudio: &hasAudio,
		Width:    0,
		Height:   0,
	}

	info := ffprobe.Resolve(context.Background(), r, prober, slog.Default())

	if exec.capturedName != "ffprobe" {
		t.Error("Prober não foi chamado apesar de Width=0 — Resolve deve sondar quando dimensões são desconhecidas")
	}
	if info.Width != 1920 || info.Height != 1080 {
		t.Errorf("esperado 1920×1080, obtido %d×%d", info.Width, info.Height)
	}
}

func TestResolve_SkipsProbeWhenDimensionsKnown(t *testing.T) {
	// Quando width e height já são conhecidos, não deve sondar.
	exec := &fakeExecutor{output: []byte(`{}`)}
	prober := ffprobe.NewProber(exec)

	r := ffprobe.Resolver{
		RTSPURL:    "rtsp://cam",
		VideoCodec: "h265",
		Width:      1280,
		Height:     720,
	}

	info := ffprobe.Resolve(context.Background(), r, prober, slog.Default())

	if exec.capturedName == "ffprobe" {
		t.Error("Prober foi chamado desnecessariamente quando dimensões já eram conhecidas")
	}
	if info.Width != 1280 || info.Height != 720 {
		t.Errorf("esperado 1280×720, obtido %d×%d", info.Width, info.Height)
	}
}
