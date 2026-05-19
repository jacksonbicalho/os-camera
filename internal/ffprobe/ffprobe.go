package ffprobe

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type StreamInfo struct {
	VideoCodec string
	HasAudio   bool
	Width      int
	Height     int
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

func Parse(data []byte) (StreamInfo, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return StreamInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	var info StreamInfo
	for _, s := range out.Streams {
		switch s.CodecType {
		case "video":
			info.VideoCodec = s.CodecName
			info.Width = s.Width
			info.Height = s.Height
		case "audio":
			info.HasAudio = true
		}
	}
	return info, nil
}

type Executor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Prober struct {
	exec Executor
}

func NewProber(exec Executor) *Prober {
	return &Prober{exec: exec}
}

func (p *Prober) Probe(ctx context.Context, url string) ([]byte, error) {
	return p.exec.Execute(ctx, "ffprobe",
		"-rtsp_transport", "tcp",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		url,
	)
}

type OSExecutor struct{}

func (e *OSExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
