package motion

import (
	"context"
	"log/slog"
	"time"

	"camera/internal/config"
	"camera/internal/ffprobe"
	"camera/internal/zones"
)

type Event struct {
	Time  time.Time
	Score float64
	Label string
	Color string
}

type Monitor struct {
	det               *detector
	reconnectInterval time.Duration
	log               *slog.Logger
	cameraID          string
	events            chan Event
	rawScores         chan Event
}

// New creates a Monitor. onEvent, if non-nil, is called for every motion event
// in addition to writing the NDJSON file. It receives the full event data
// including frame filename and bounding box.
func New(cam config.CameraConfig, stream ffprobe.StreamInfo, cfg config.MotionConfig, storagePath string, reconnectInterval time.Duration, log *slog.Logger, getZones func() []zones.Zone, onEvent func(cameraID string, t time.Time, score float64, frame, label, color string, bbox BBox)) *Monitor {
	scaledW := cfg.CaptureWidth
	scaledH := cfg.CaptureHeight
	if scaledW < 1 || scaledH < 1 {
		scaledW = stream.Width / 4
		scaledH = stream.Height / 4
	}
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
	effective := config.MotionConfig{Enabled: true, Threshold: threshold, FPS: fps, CooldownSeconds: cfg.CooldownSeconds}

	events := make(chan Event, 64)
	notify := func(ev Event) {
		select {
		case events <- ev:
		default:
		}
	}

	rawScores := make(chan Event, 256)
	notifyRaw := func(ev Event) {
		select {
		case rawScores <- ev:
		default:
		}
	}

	cmd := newFFmpegFrameCommander()
	st := newStore(storagePath, onEvent)
	det := newDetector(cam.ID, cam.EffectiveMotionURL(), scaledW, scaledH, effective, cmd, st, log, notify, notifyRaw, getZones)
	// The pipe delivers full-res frames; the diff runs on a downscaled grayscale
	// copy while the event snapshot keeps the original full resolution.
	det.fullW = stream.Width
	det.fullH = stream.Height

	return &Monitor{
		det:               det,
		reconnectInterval: reconnectInterval,
		log:               log,
		cameraID:          cam.ID,
		events:            events,
		rawScores:         rawScores,
	}
}

func (m *Monitor) Events() <-chan Event {
	return m.events
}

func (m *Monitor) RawScores() <-chan Event {
	return m.rawScores
}

// RegisterInspector registers a bounding box region and returns an id and a
// channel that receives the per-frame diff score for that region. Call
// UnregisterInspector when done to free the registry entry.
func (m *Monitor) RegisterInspector(bbox BBox) (string, <-chan float64) {
	return m.det.registerInspector(bbox)
}

// UnregisterInspector removes the inspector from the registry. The channel
// returned by RegisterInspector is NOT closed — the caller must stop reading.
func (m *Monitor) UnregisterInspector(id string) {
	m.det.unregisterInspector(id)
}

// ReloadZones refreshes the detector's in-memory zone cache from the source.
// Call after zones are edited (e.g. via the API) so the change takes effect
// without restarting the monitor. Safe for concurrent use.
func (m *Monitor) ReloadZones() {
	m.det.reloadZones()
}

func (m *Monitor) Run(ctx context.Context) {
	defer close(m.events)
	defer close(m.rawScores)
	for {
		m.det.processFrames(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.reconnectInterval):
			m.log.Info("motion: reconnecting", "camera", m.cameraID)
		}
	}
}
