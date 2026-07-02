// Package live implements low-latency WebRTC delivery of a camera's live feed.
// It pulls H.264 RTP from an RTSP source and repackages it to WebRTC viewers
// without transcoding: a Publisher holds a single shared H.264 track and fans
// each RTP packet out to every connected PeerConnection. This targets
// sub-second latency at near-zero CPU (no decode), unlike the segment-based
// HLS pipeline which floors around 5-6s.
package live

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

// Source is an RTP source of H.264 video. ReadRTP forwards each packet to
// onPacket until ctx is cancelled or a fatal error occurs; it returns nil on
// clean context cancellation. Defined here (consumer side) so tests can inject
// a fake that emits synthetic packets, with no camera and no network.
type Source interface {
	ReadRTP(ctx context.Context, onPacket func(*rtp.Packet)) error
}

// Publisher owns a single H.264 WebRTC track for one camera and fans the RTP
// stream out to all connected viewer PeerConnections.
type Publisher struct {
	cameraID string
	source   Source
	track    *webrtc.TrackLocalStaticRTP
	api      *webrtc.API
	log      *slog.Logger

	mu  sync.Mutex
	pcs map[*webrtc.PeerConnection]struct{}
}

// NewPublisher builds a Publisher with an H.264 track. The source is read only
// once Run is called.
func NewPublisher(cameraID string, source Source, log *slog.Logger) (*Publisher, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, fmt.Errorf("register codecs: %w", err)
	}
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video", "os-camera-"+cameraID,
	)
	if err != nil {
		return nil, fmt.Errorf("create track: %w", err)
	}
	return &Publisher{
		cameraID: cameraID,
		source:   source,
		track:    track,
		api:      webrtc.NewAPI(webrtc.WithMediaEngine(m)),
		log:      log,
		pcs:      make(map[*webrtc.PeerConnection]struct{}),
	}, nil
}

// Run reads the source and writes every RTP packet to the shared track,
// reconnecting after failures until ctx is cancelled. On exit it closes all
// viewer connections. Mirrors the reconnect loop of streaming.HLSStreamer.
func (p *Publisher) Run(ctx context.Context, reconnect time.Duration) {
	defer p.closeAll()
	for {
		err := p.source.ReadRTP(ctx, func(pkt *rtp.Packet) {
			if werr := p.track.WriteRTP(pkt); werr != nil && !errors.Is(werr, io.ErrClosedPipe) {
				p.log.Debug("live: write rtp", "camera", p.cameraID, "error", werr)
			}
		})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			p.log.Warn("live: source ended", "camera", p.cameraID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnect):
			p.log.Info("live: reconnecting source", "camera", p.cameraID)
		}
	}
}

// Negotiate completes the WebRTC handshake for one viewer: it creates a
// PeerConnection, attaches the shared track and returns the SDP answer. The
// connection is tracked and closed automatically when it fails or disconnects.
func (p *Publisher) Negotiate(offer webrtc.SessionDescription) (webrtc.SessionDescription, error) {
	pc, err := p.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("new peer connection: %w", err)
	}
	if _, err := pc.AddTrack(p.track); err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, fmt.Errorf("add track: %w", err)
	}
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateFailed,
			webrtc.PeerConnectionStateClosed,
			webrtc.PeerConnectionStateDisconnected:
			p.remove(pc)
		}
	})
	if err := pc.SetRemoteDescription(offer); err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, fmt.Errorf("set remote description: %w", err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, fmt.Errorf("create answer: %w", err)
	}
	// Gather all ICE candidates before returning: signaling here is a single
	// offer/answer exchange (no trickle), so the answer must carry candidates.
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		_ = pc.Close()
		return webrtc.SessionDescription{}, fmt.Errorf("set local description: %w", err)
	}
	<-gatherComplete

	p.mu.Lock()
	p.pcs[pc] = struct{}{}
	p.mu.Unlock()

	return *pc.LocalDescription(), nil
}

// Sessions returns the number of currently connected viewers.
func (p *Publisher) Sessions() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.pcs)
}

func (p *Publisher) remove(pc *webrtc.PeerConnection) {
	p.mu.Lock()
	_, ok := p.pcs[pc]
	delete(p.pcs, pc)
	p.mu.Unlock()
	if ok {
		_ = pc.Close()
	}
}

func (p *Publisher) closeAll() {
	p.mu.Lock()
	pcs := make([]*webrtc.PeerConnection, 0, len(p.pcs))
	for pc := range p.pcs {
		pcs = append(pcs, pc)
	}
	p.pcs = make(map[*webrtc.PeerConnection]struct{})
	p.mu.Unlock()
	for _, pc := range pcs {
		_ = pc.Close()
	}
}
