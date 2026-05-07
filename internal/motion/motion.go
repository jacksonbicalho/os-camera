package motion

import (
	"context"
	"log/slog"
	"time"

	"camera/internal/config"
	"camera/internal/ffprobe"
)

type Event struct {
	Time  time.Time
	Score float64
}

type Monitor struct {
	det               *detector
	reconnectInterval time.Duration
	log               *slog.Logger
	cameraID          string
	events            chan Event
}

func New(cam config.CameraConfig, stream ffprobe.StreamInfo, cfg config.MotionConfig, storagePath string, reconnectInterval time.Duration, log *slog.Logger) *Monitor {
	scaledW := stream.Width / 4
	scaledH := stream.Height / 4
	if scaledW < 1 {
		scaledW = 1
	}
	if scaledH < 1 {
		scaledH = 1
	}

	fps := cfg.FPS
	if fps < 1 {
		fps = 2
	}
	threshold := cfg.Threshold
	if threshold <= 0 {
		threshold = 0.02
	}
	effective := config.MotionConfig{Enabled: true, Threshold: threshold, FPS: fps}

	events := make(chan Event, 64)
	notify := func(ev Event) {
		select {
		case events <- ev:
		default:
		}
	}

	cmd := newFFmpegFrameCommander()
	st := newStore(storagePath)
	det := newDetector(cam.ID, cam.RTSPURL, scaledW, scaledH, effective, cmd, st, log, notify)

	return &Monitor{
		det:               det,
		reconnectInterval: reconnectInterval,
		log:               log,
		cameraID:          cam.ID,
		events:            events,
	}
}

func (m *Monitor) Events() <-chan Event {
	return m.events
}

func (m *Monitor) Run(ctx context.Context) {
	defer close(m.events)
	for {
		m.det.processFrames()
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.reconnectInterval):
			m.log.Info("motion: reconnecting", "camera", m.cameraID)
		}
	}
}
