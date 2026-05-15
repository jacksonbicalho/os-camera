package ffprobe

import (
	"context"
	"log/slog"
	"time"
)

// Resolver groups stream field overrides used when probing is not needed.
type Resolver struct {
	VideoCodec string
	HasAudio   *bool
	Width      int
	Height     int
	RTSPURL    string
}

// Resolve probes the RTSP stream when all stream fields are unset ("auto"),
// then merges any explicit overrides on top of the probed values.
func Resolve(ctx context.Context, r Resolver, prober *Prober, log *slog.Logger) StreamInfo {
	needsProbe := r.VideoCodec == "" && r.HasAudio == nil && r.Width == 0 && r.Height == 0

	var info StreamInfo
	if needsProbe {
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		raw, err := prober.Probe(probeCtx, r.RTSPURL)
		if err != nil {
			log.Warn("ffprobe failed, assuming audio present", "url", r.RTSPURL, "error", err)
			info.HasAudio = true
			return info
		}
		info, err = Parse(raw)
		if err != nil {
			log.Warn("ffprobe parse failed, assuming audio present", "url", r.RTSPURL, "error", err)
			info.HasAudio = true
			return info
		}
		log.Info("stream probed", "url", r.RTSPURL, "codec", info.VideoCodec,
			"has_audio", info.HasAudio, "width", info.Width, "height", info.Height)
	}

	if r.VideoCodec != "" {
		info.VideoCodec = r.VideoCodec
	}
	if r.HasAudio != nil {
		info.HasAudio = *r.HasAudio
	}
	if r.Width != 0 {
		info.Width = r.Width
	}
	if r.Height != 0 {
		info.Height = r.Height
	}
	return info
}
