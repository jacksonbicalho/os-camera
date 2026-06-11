package motion

import (
	"context"
	"fmt"
	"image/color"
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
	width      int // diff resolution (downscaled grayscale used for diff/bbox)
	height     int
	fullW      int // full-res pipe + snapshot resolution (0 → same as width/height)
	fullH      int
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

	zonesMu     sync.RWMutex
	cachedZones []zones.Zone
	zonesLoaded bool

	bg []byte // background model (diff resolution) for bbox localization

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

// fullDims returns the full-resolution pipe/snapshot dimensions, falling back to
// the diff resolution when no separate full resolution was configured (tests).
func (d *detector) fullDims() (int, int) {
	if d.fullW > 0 && d.fullH > 0 {
		return d.fullW, d.fullH
	}
	return d.width, d.height
}

// zones returns the cached motion zones, loading them from the source on first
// access. Zones change rarely (only when edited via the API), so caching avoids
// hitting the database on every frame. Call reloadZones to refresh after a change.
func (d *detector) zones() []zones.Zone {
	d.zonesMu.RLock()
	if d.zonesLoaded {
		zs := d.cachedZones
		d.zonesMu.RUnlock()
		return zs
	}
	d.zonesMu.RUnlock()
	return d.reloadZones()
}

// reloadZones re-fetches zones from the source and replaces the cache. Safe for
// concurrent use (e.g. the detector goroutine and an API save handler).
func (d *detector) reloadZones() []zones.Zone {
	var zs []zones.Zone
	if d.getZones != nil {
		zs = d.getZones()
	}
	d.zonesMu.Lock()
	d.cachedZones = zs
	d.zonesLoaded = true
	d.zonesMu.Unlock()
	return zs
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

// processFrames starts the frame commander, reads full-resolution RGB frames
// until EOF or ctx cancellation, downsamples each to grayscale for the diff, and
// records a motion event whenever the diff between consecutive frames exceeds the
// configured threshold. The snapshot for every event is the very frame that
// triggered it (annotated in place), so the bbox always matches the subject.
func (d *detector) processFrames(ctx context.Context) {
	fw, fh := d.fullDims()
	d.log.Debug("motion: starting frame capture", "camera", d.cameraID, "fullW", fw, "fullH", fh, "diffW", d.width, "diffH", d.height, "fps", d.cfg.FPS)
	proc, err := d.commander.Start(d.url, fw, fh, d.cfg.FPS)
	if err != nil {
		d.log.Error("motion: failed to start frame capture", "camera", d.cameraID, "error", err)
		return
	}
	go func() {
		<-ctx.Done()
		proc.Terminate()
	}()
	defer func() {
		proc.Terminate()
		proc.Wait()
	}()

	rgbBuf := make([]byte, fw*fh*3)
	var prev []byte // grayscale at diff resolution
	var frameCount int

	for {
		if _, err := io.ReadFull(proc, rgbBuf); err != nil {
			return
		}
		cur := downscaleRGBToGray(rgbBuf, fw, fh, d.width, d.height)

		if prev != nil {
			zs := d.zones()

			// Initialize background from the first prev frame seen.
			if d.bg == nil {
				d.bg = make([]byte, len(prev))
				copy(d.bg, prev)
			}

			score := diffFramesMasked(prev, cur, d.width, d.height, zs)
			ts := d.now()
			if d.notifyRaw != nil {
				d.notifyRaw(Event{Time: ts, Score: score})
			}
			if score >= d.cfg.Threshold && d.cooldownElapsed(ts) {
				// Background subtraction localizes WHERE the object IS in cur.
				bbox, bboxFound := computeBBox(d.bg, cur, d.width, d.height, zs)
				d.lastEvent = ts
				d.saveSnapshot(ts, score, bbox, bboxFound, rgbBuf, fw, fh, ColorGlobal, "", "")
			}

			d.evaluateDetectZones(d.bg, prev, cur, zs, ts, frameCount, rgbBuf, fw, fh)
			d.notifyInspectors(prev, cur)

			// Advance background toward current frame only when idle (no motion),
			// so moving objects are not absorbed into the background model.
			if score < d.cfg.Threshold {
				updateBackground(d.bg, cur, 0.05)
			}
		}
		prev = cur
		frameCount++
	}
}

func (d *detector) evaluateDetectZones(bg, prev, cur []byte, zs []zones.Zone, ts time.Time, frameCount int, rgb []byte, fw, fh int) {
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
			bbox, bboxFound := computeBBoxInZone(bg, cur, d.width, d.height, dz)
			annColor := ColorDetect
			if dz.Color != "" {
				annColor = hexToNRGBA(dz.Color)
			}
			d.zoneLastEv[i] = ts
			d.saveSnapshot(ts, zScore, bbox, bboxFound, rgb, fw, fh, annColor, dz.Label, dz.Color)
		}
	}
}

// saveSnapshot annotates the full-res RGB frame that triggered the event with the
// bbox and score, persists the JPEG and records the event. It runs synchronously:
// the frame is already in memory, so there is no async RTSP grab and the snapshot
// always reflects the exact instant of detection.
func (d *detector) saveSnapshot(ts time.Time, score float64, bbox BBox, drawRect bool, rgb []byte, fw, fh int, annColor color.NRGBA, label, recColor string) {
	jpegData := annotateRGBFrame(rgb, fw, fh, bbox, score, annColor, drawRect)
	frameName, _, err := d.st.saveJPEG(d.cameraID, ts, jpegData)
	if err != nil {
		d.log.Warn("motion: failed to save snapshot", "camera", d.cameraID, "error", err)
	}
	if err := d.st.record(d.cameraID, ts, score, frameName, label, recColor, bbox); err != nil {
		d.log.Error("motion: failed to record event", "camera", d.cameraID, "error", err)
	} else if d.notify != nil {
		d.notify(Event{Time: ts, Score: score, Label: label, Color: recColor})
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
