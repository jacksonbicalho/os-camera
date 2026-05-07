package motion

import (
	"io"
	"log/slog"
	"time"

	"camera/internal/config"
)

type frameProcess interface {
	Read(p []byte) (int, error)
	Terminate() error
	Wait() error
}

type frameCommander interface {
	Start(url string, width, height, fps int) (frameProcess, error)
}

type detector struct {
	cameraID   string
	url        string
	width      int
	height     int
	cfg        config.MotionConfig
	commander  frameCommander
	st         *store
	log        *slog.Logger
	notify     func(Event)
	notifyRaw  func(Event)
}

func newDetector(cameraID, url string, width, height int, cfg config.MotionConfig, cmd frameCommander, st *store, log *slog.Logger, notify func(Event), notifyRaw func(Event)) *detector {
	return &detector{
		cameraID:  cameraID,
		url:       url,
		width:     width,
		height:    height,
		cfg:       cfg,
		commander: cmd,
		st:        st,
		log:       log,
		notify:    notify,
		notifyRaw: notifyRaw,
	}
}

// processFrames starts the frame commander, reads frames until EOF,
// and records a motion event whenever the diff between consecutive frames
// exceeds the configured threshold.
func (d *detector) processFrames() {
	d.log.Debug("motion: starting frame capture", "camera", d.cameraID, "width", d.width, "height", d.height, "fps", d.cfg.FPS)
	proc, err := d.commander.Start(d.url, d.width, d.height, d.cfg.FPS)
	if err != nil {
		d.log.Error("motion: failed to start frame capture", "camera", d.cameraID, "error", err)
		return
	}
	defer func() {
		proc.Terminate()
		proc.Wait()
	}()

	frameSize := d.width * d.height
	buf := make([]byte, frameSize)
	var prev []byte

	for {
		if _, err := io.ReadFull(proc, buf); err != nil {
			return
		}
		cur := make([]byte, frameSize)
		copy(cur, buf)

		if prev != nil {
			score := diffFrames(prev, cur)
			ts := time.Now().UTC()
			if d.notifyRaw != nil {
				d.notifyRaw(Event{Time: ts, Score: score})
			}
			if score >= d.cfg.Threshold {
				if err := d.st.record(d.cameraID, ts, score); err != nil {
					d.log.Error("motion: failed to record event", "camera", d.cameraID, "error", err)
				} else if d.notify != nil {
					d.notify(Event{Time: ts, Score: score})
				}
			}
		}
		prev = cur
	}
}
