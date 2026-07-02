package live

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// ErrNoH264 is returned when the RTSP stream exposes no H.264 media. WebRTC in
// browsers requires H.264, so a stream without it cannot be repackaged and the
// caller should fall back to HLS.
var ErrNoH264 = errors.New("rtsp stream has no h264 media")

// RTSPSource pulls an H.264 stream from an RTSP URL over TCP and forwards its
// RTP packets unchanged. It implements Source. Transport is TCP to match the
// recorder/HLS pipelines (interleaved, firewall-friendly, no UDP reordering).
type RTSPSource struct {
	url string
	log *slog.Logger
}

// NewRTSPSource returns a Source backed by the given RTSP URL.
func NewRTSPSource(url string, log *slog.Logger) *RTSPSource {
	return &RTSPSource{url: url, log: log}
}

func (s *RTSPSource) ReadRTP(ctx context.Context, onPacket func(*rtp.Packet)) error {
	u, err := base.ParseURL(s.url)
	if err != nil {
		return fmt.Errorf("parse rtsp url: %w", err)
	}

	transport := gortsplib.TransportTCP
	c := &gortsplib.Client{Transport: &transport}
	if err := c.Start(u.Scheme, u.Host); err != nil {
		return fmt.Errorf("rtsp start: %w", err)
	}
	defer c.Close()

	desc, _, err := c.Describe(u)
	if err != nil {
		return fmt.Errorf("rtsp describe: %w", err)
	}

	var forma *format.H264
	medi := desc.FindFormat(&forma)
	if medi == nil {
		return ErrNoH264
	}
	if _, err := c.Setup(desc.BaseURL, medi, 0, 0); err != nil {
		return fmt.Errorf("rtsp setup: %w", err)
	}
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		onPacket(pkt)
	})
	if _, err := c.Play(nil); err != nil {
		return fmt.Errorf("rtsp play: %w", err)
	}

	// c.Wait blocks until a read error; race it against context cancellation so
	// the deferred Close tears the session down on shutdown.
	errCh := make(chan error, 1)
	go func() { errCh <- c.Wait() }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}
