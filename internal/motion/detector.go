package motion

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"camera/internal/config"
	"camera/internal/zones"
)

var detectorInspSeq atomic.Int64

type inspectorEntry struct {
	bbox BBox
	ch   chan float64
}

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
	getZones   func() []zones.Zone
	now        func() time.Time
	lastEvent  time.Time
	zoneLastEv []time.Time

	inspMu     sync.Mutex
	inspectors map[string]inspectorEntry
}

func newDetector(cameraID, url string, width, height int, cfg config.MotionConfig, cmd frameCommander, st *store, log *slog.Logger, notify func(Event), notifyRaw func(Event), getZones func() []zones.Zone) *detector {
	return &detector{
		cameraID:   cameraID,
		url:        url,
		width:      width,
		height:     height,
		cfg:        cfg,
		commander:  cmd,
		st:         st,
		log:        log,
		notify:     notify,
		notifyRaw:  notifyRaw,
		getZones:   getZones,
		now:        func() time.Time { return time.Now().UTC() },
		inspectors: make(map[string]inspectorEntry),
	}
}

func (d *detector) cooldownElapsed(now time.Time) bool {
	if d.cfg.CooldownSeconds <= 0 {
		return true
	}
	return now.Sub(d.lastEvent) >= time.Duration(d.cfg.CooldownSeconds)*time.Second
}

func (d *detector) registerInspector(bbox BBox) (string, <-chan float64) {
	id := fmt.Sprintf("insp-%d", detectorInspSeq.Add(1))
	ch := make(chan float64, 1)
	d.inspMu.Lock()
	d.inspectors[id] = inspectorEntry{bbox: bbox, ch: ch}
	d.inspMu.Unlock()
	return id, ch
}

func (d *detector) unregisterInspector(id string) {
	d.inspMu.Lock()
	delete(d.inspectors, id)
	d.inspMu.Unlock()
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
	var frameCount int

	for {
		if _, err := io.ReadFull(proc, buf); err != nil {
			return
		}
		cur := make([]byte, frameSize)
		copy(cur, buf)

		if prev != nil {
			var zs []zones.Zone
			if d.getZones != nil {
				zs = d.getZones()
			}
			score := diffFramesMasked(prev, cur, d.width, d.height, zs)
			ts := d.now()
			if d.notifyRaw != nil {
				d.notifyRaw(Event{Time: ts, Score: score})
			}
			if score >= d.cfg.Threshold && d.cooldownElapsed(ts) {
				bbox := computeBBox(prev, cur, d.width, d.height, zs)
				jpegData := annotateFrame(cur, d.width, d.height, bbox, score, ColorGlobal)
				frameName, saveErr := d.st.saveJPEG(d.cameraID, ts, jpegData)
				if saveErr != nil {
					d.log.Warn("motion: failed to save snapshot", "camera", d.cameraID, "error", saveErr)
				}
				if err := d.st.record(d.cameraID, ts, score, frameName, "", "", bbox); err != nil {
					d.log.Error("motion: failed to record event", "camera", d.cameraID, "error", err)
				} else if d.notify != nil {
					d.lastEvent = ts
					d.notify(Event{Time: ts, Score: score})
				}
			}

			d.evaluateDetectZones(prev, cur, zs, ts, frameCount)
			d.notifyInspectors(prev, cur)
		}
		prev = cur
		frameCount++
	}
}

func (d *detector) evaluateDetectZones(prev, cur []byte, zs []zones.Zone, ts time.Time, frameCount int) {
	var detect []zones.Zone
	for _, z := range zs {
		if !z.IsExclude() {
			detect = append(detect, z)
		}
	}
	if len(detect) != len(d.zoneLastEv) {
		d.zoneLastEv = make([]time.Time, len(detect))
	}
	for i, dz := range detect {
		if dz.FPS > 0 && d.cfg.FPS > dz.FPS {
			skip := d.cfg.FPS / dz.FPS
			if frameCount%skip != 0 {
				continue
			}
		}
		zScore := diffFramesForZoneScaled(prev, cur, d.width, d.height, dz)
		thr := dz.Threshold
		if thr == 0 {
			thr = d.cfg.Threshold
		}
		cd := dz.CooldownSeconds
		if cd == 0 {
			cd = d.cfg.CooldownSeconds
		}
		cooldownOk := cd <= 0 || ts.Sub(d.zoneLastEv[i]) >= time.Duration(cd)*time.Second
		if zScore >= thr && cooldownOk {
			d.zoneLastEv[i] = ts
			bbox := BBox{X: dz.X, Y: dz.Y, W: dz.W, H: dz.H}
			zoneColor := ColorDetect
			if dz.Color != "" {
				zoneColor = hexToNRGBA(dz.Color)
			}
			jpegData := annotateFrame(cur, d.width, d.height, bbox, zScore, zoneColor)
			frameName, saveErr := d.st.saveJPEG(d.cameraID, ts, jpegData)
			if saveErr != nil {
				d.log.Warn("motion: failed to save zone snapshot", "camera", d.cameraID, "zone", dz.Label, "error", saveErr)
			}
			if err := d.st.record(d.cameraID, ts, zScore, frameName, dz.Label, dz.Color, bbox); err != nil {
				d.log.Error("motion: failed to record zone event", "camera", d.cameraID, "zone", dz.Label, "error", err)
			} else if d.notify != nil {
				d.notify(Event{Time: ts, Score: zScore, Label: dz.Label, Color: dz.Color})
			}
		}
	}
}

func (d *detector) notifyInspectors(prev, cur []byte) {
	d.inspMu.Lock()
	snapshot := make([]inspectorEntry, 0, len(d.inspectors))
	for _, e := range d.inspectors {
		snapshot = append(snapshot, e)
	}
	d.inspMu.Unlock()

	for _, e := range snapshot {
		score := diffFramesForZone(prev, cur, d.width, d.height, zones.Zone{
			X: e.bbox.X, Y: e.bbox.Y, W: e.bbox.W, H: e.bbox.H,
		})
		select {
		case e.ch <- score:
		default:
		}
	}
}
