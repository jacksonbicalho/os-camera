package live

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeSource emits synthetic H.264 RTP packets until the context is cancelled,
// standing in for a real RTSP stream so tests need no camera or network.
type fakeSource struct {
	mu       sync.Mutex
	returned bool
}

func (f *fakeSource) ReadRTP(ctx context.Context, onPacket func(*rtp.Packet)) error {
	defer func() {
		f.mu.Lock()
		f.returned = true
		f.mu.Unlock()
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var seq uint16
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			seq++
			onPacket(&rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    96,
					SequenceNumber: seq,
					Timestamp:      uint32(seq) * 3000,
				},
				Payload: []byte{0x00, 0x00, 0x01, 0x65, 0x00},
			})
		}
	}
}

func (f *fakeSource) hasReturned() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.returned
}

// newViewer creates a recvonly video PeerConnection (the browser side) with a
// fully-gathered offer, ready to hand to Publisher.Negotiate.
func newViewer(t *testing.T) (*webrtc.PeerConnection, webrtc.SessionDescription) {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("client peer connection: %v", err)
	}
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatalf("add transceiver: %v", err)
	}
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		t.Fatalf("create offer: %v", err)
	}
	gather := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(offer); err != nil {
		t.Fatalf("set local description: %v", err)
	}
	<-gather
	return pc, *pc.LocalDescription()
}

func TestPublisherNegotiatesAndForwardsRTP(t *testing.T) {
	pub, err := NewPublisher("cam1", &fakeSource{}, discardLogger())
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go pub.Run(ctx, time.Second)

	client, offer := newViewer(t)
	defer client.Close()

	gotRTP := make(chan struct{}, 1)
	client.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		for {
			if _, _, err := tr.ReadRTP(); err != nil {
				return
			}
			select {
			case gotRTP <- struct{}{}:
			default:
			}
		}
	})

	answer, err := pub.Negotiate(offer)
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if !strings.Contains(strings.ToLower(answer.SDP), "h264") {
		t.Fatalf("answer SDP missing h264 codec:\n%s", answer.SDP)
	}
	if err := client.SetRemoteDescription(answer); err != nil {
		t.Fatalf("set remote description: %v", err)
	}

	select {
	case <-gotRTP:
	case <-time.After(15 * time.Second):
		t.Fatal("did not receive forwarded RTP within timeout")
	}
}

func TestPublisherClosesSessionAndSourceOnCancel(t *testing.T) {
	src := &fakeSource{}
	pub, err := NewPublisher("cam1", src, discardLogger())
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { pub.Run(ctx, time.Second); close(done) }()

	client, offer := newViewer(t)
	defer client.Close()
	answer, err := pub.Negotiate(offer)
	if err != nil {
		t.Fatalf("negotiate: %v", err)
	}
	if err := client.SetRemoteDescription(answer); err != nil {
		t.Fatalf("set remote description: %v", err)
	}
	if pub.Sessions() != 1 {
		t.Fatalf("expected 1 session after negotiate, got %d", pub.Sessions())
	}

	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
	if !src.hasReturned() {
		t.Fatal("source ReadRTP did not return after cancel (leak)")
	}
	if pub.Sessions() != 0 {
		t.Fatalf("expected 0 sessions after cancel, got %d", pub.Sessions())
	}
}
