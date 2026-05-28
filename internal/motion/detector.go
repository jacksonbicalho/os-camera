package motion

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

type snapshotGrabber interface {
	Grab(ctx context.Context, rtspURL, destPath string) error
}

type detector struct {
	cameraID   string
	url        string
	width      int
	height     int
	cfg        config.MotionConfig
	commander  frameCommander
	grabber    snapshotGrabber
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

// processFrames starts the frame commander, reads frames until EOF or ctx
// cancellation, and records a motion event whenever the diff between consecutive
// frames exceeds the configured threshold.
func (d *detector) processFrames(ctx context.Context) {
	d.log.Debug("motion: starting frame capture", "camera", d.cameraID, "width", d.width, "height", d.height, "fps", d.cfg.FPS)
	proc, err := d.commander.Start(d.url, d.width, d.height, d.cfg.FPS)
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
				bbox, bboxFound := computeBBox(prev, cur, d.width, d.height, zs)
				d.lastEvent = ts // update cooldown immediately before goroutine
				if d.grabber != nil {
					lowRes := annotateFrame(cur, d.width, d.height, bbox, score, ColorGlobal, bboxFound)
					go d.recordWithHighRes(ctx, ts, score, bbox, lowRes, "", "")
				} else {
					jpegData := annotateFrame(cur, d.width, d.height, bbox, score, ColorGlobal, bboxFound)
					frameName, _, saveErr := d.st.saveJPEG(d.cameraID, ts, jpegData)
					if saveErr != nil {
						d.log.Warn("motion: failed to save snapshot", "camera", d.cameraID, "error", saveErr)
					}
					if err := d.st.record(d.cameraID, ts, score, frameName, "", "", bbox); err != nil {
						d.log.Error("motion: failed to record event", "camera", d.cameraID, "error", err)
					} else if d.notify != nil {
						d.notify(Event{Time: ts, Score: score})
					}
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
			bbox, bboxFound := computeBBoxInZone(prev, cur, d.width, d.height, dz)
			zoneColor := ColorDetect
			if dz.Color != "" {
				zoneColor = hexToNRGBA(dz.Color)
			}
			d.zoneLastEv[i] = ts // update cooldown immediately before goroutine
			if d.grabber != nil {
				lowRes := annotateFrame(cur, d.width, d.height, bbox, zScore, zoneColor, bboxFound)
				go d.recordWithHighRes(context.Background(), ts, zScore, bbox, lowRes, dz.Label, dz.Color)
			} else {
				jpegData := annotateFrame(cur, d.width, d.height, bbox, zScore, zoneColor, bboxFound)
				frameName, _, saveErr := d.st.saveJPEG(d.cameraID, ts, jpegData)
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
}

// recordWithHighRes grabs a full-resolution JPEG from the RTSP stream, saves it
// (falling back to the provided low-res data on failure), then records the event
// and fires the notify callback. Running in a goroutine so the detector loop is
// not blocked.
func (d *detector) recordWithHighRes(ctx context.Context, ts time.Time, score float64, bbox BBox, lowRes []byte, label, zoneColor string) {
	grabCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Build a temp path for the high-res grab; saveJPEG will finalise it.
	dir := filepath.Join(d.st.basePath, d.cameraID, ts.UTC().Format("2006/01/02"))
	_ = os.MkdirAll(dir, 0755)
	tmpPath := filepath.Join(dir, ts.UTC().Format("20060102150405")+"_motion_hires.jpg")

	var frameName string
	c := ColorGlobal
	if zoneColor != "" {
		c = hexToNRGBA(zoneColor)
	}
	if err := d.grabber.Grab(grabCtx, d.url, tmpPath); err != nil {
		d.log.Warn("motion: high-res grab failed, using low-res snapshot", "camera", d.cameraID, "err", err)
		var saveErr error
		frameName, _, saveErr = d.st.saveJPEG(d.cameraID, ts, lowRes)
		if saveErr != nil {
			d.log.Warn("motion: failed to save low-res snapshot", "camera", d.cameraID, "error", saveErr)
		}
	} else {
		finalName := ts.UTC().Format("20060102150405") + "_motion.jpg"
		finalPath := filepath.Join(dir, finalName)
		raw, readErr := os.ReadFile(tmpPath)
		os.Remove(tmpPath)
		if readErr != nil {
			d.log.Warn("motion: failed to read high-res grab", "camera", d.cameraID, "err", readErr)
			frameName, _, _ = d.st.saveJPEG(d.cameraID, ts, lowRes)
		} else {
			annotated, annErr := annotateJPEGBytes(raw, bbox, score, c, true)
			if annErr != nil {
				d.log.Warn("motion: failed to annotate high-res JPEG, using raw grab", "camera", d.cameraID, "err", annErr)
				annotated = raw
			}
			if err := os.WriteFile(finalPath, annotated, 0644); err != nil {
				d.log.Warn("motion: failed to write high-res snapshot", "camera", d.cameraID, "err", err)
				frameName, _, _ = d.st.saveJPEG(d.cameraID, ts, lowRes)
			} else {
				frameName = finalName
			}
		}
	}

	if err := d.st.record(d.cameraID, ts, score, frameName, label, zoneColor, bbox); err != nil {
		d.log.Error("motion: failed to record event", "camera", d.cameraID, "error", err)
	} else if d.notify != nil {
		d.notify(Event{Time: ts, Score: score, Label: label, Color: zoneColor})
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
