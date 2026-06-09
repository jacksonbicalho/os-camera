package recorder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/ffprobe"
)

type Recorder struct {
	camera    config.CameraConfig
	storage   config.StorageConfig
	stream    ffprobe.StreamInfo
	commander exec.Commander
	log       *slog.Logger
	process   exec.Process
	now       func() time.Time
}

func NewRecorder(camera config.CameraConfig, storage config.StorageConfig, stream ffprobe.StreamInfo, commander exec.Commander, log *slog.Logger) *Recorder {
	return &Recorder{
		camera:    camera,
		storage:   storage,
		stream:    stream,
		commander: commander,
		log:       log,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (r *Recorder) Start(now time.Time) error {
	dir := OutputDir(r.storage.Path, r.camera.ID, now)
	r.log.Debug("creating output directory", "path", dir, "camera", r.camera.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	pattern := OutputPattern(r.storage.Path, r.camera.ID, now)
	duration := int(r.camera.EffectiveChunkDuration().Seconds())
	r.log.Debug("starting ffmpeg", "camera", r.camera.ID, "pattern", pattern, "chunk_duration", duration)
	args := []string{"-rtsp_transport", "tcp", "-i", r.camera.RTSPURL}
	if r.needsTranscode() {
		r.log.Warn("transcoding video to h264", "camera", r.camera.ID, "source_codec", r.stream.VideoCodec, "mode", r.camera.RecordVideoMode)
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency")
		if r.stream.HasAudio {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-an")
		}
	} else {
		args = append(args, "-c", "copy")
		if !r.stream.HasAudio {
			args = append(args, "-an")
		}
	}
	args = append(args,
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", duration),
		"-segment_format", "mp4",
		"-reset_timestamps", "1",
		"-avoid_negative_ts", "make_zero",
		"-strftime", "1",
		pattern,
	)
	proc, err := r.commander.Start("ffmpeg", args...)
	if err != nil {
		return err
	}
	r.process = proc
	r.log.Info("recording started", "camera", r.camera.ID)
	return nil
}

func (r *Recorder) needsTranscode() bool {
	switch r.camera.RecordVideoMode {
	case "h264":
		return true
	case "copy":
		return false
	default: // "auto" or empty
		return r.stream.VideoCodec != "" && r.stream.VideoCodec != "h264"
	}
}

func (r *Recorder) Run(ctx context.Context, reconnect time.Duration) {
	for {
		start := r.now()
		if err := r.Start(start); err != nil {
			r.log.Error("recorder: failed to start", "camera", r.camera.ID, "error", err)
		} else {
			exited := make(chan struct{})
			proc := r.process
			go func() { proc.Wait(); close(exited) }()
			// ffmpeg bakes the day directory into its output pattern at start
			// time; it never rolls over on its own. Restart the session at UTC
			// midnight so the next day's chunks land in the next day's folder.
			rollover := time.After(DurationUntilNextDay(start))
			select {
			case <-ctx.Done():
				r.Stop()
				<-exited
				return
			case <-exited:
				r.log.Warn("recorder: process exited unexpectedly", "camera", r.camera.ID)
			case <-rollover:
				r.log.Info("recorder: rolling output directory at day boundary", "camera", r.camera.ID)
				r.Stop()
				<-exited
				continue // restart immediately on the new day, skipping the reconnect backoff
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(reconnect):
			r.log.Info("recorder: reconnecting", "camera", r.camera.ID)
		}
	}
}

func (r *Recorder) Stop() {
	if r.process == nil {
		return
	}
	r.log.Info("stopping recorder, finalizing chunk", "camera", r.camera.ID)
	r.process.Terminate()
	r.process.Wait()
}

// DurationUntilNextDay returns how long until the next UTC midnight after t.
// At exactly midnight it returns a full 24h (never zero), so the rollover timer
// always schedules a meaningful wait.
func DurationUntilNextDay(t time.Time) time.Duration {
	t = t.UTC()
	nextMidnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	return nextMidnight.Sub(t)
}

func OutputDir(storagePath, cameraID string, t time.Time) string {
	return fmt.Sprintf("%s/%s/%s", storagePath, cameraID, t.UTC().Format("2006/01/02"))
}

func OutputPattern(storagePath, cameraID string, t time.Time) string {
	return fmt.Sprintf("%s/%%Y%%m%%d%%H%%M%%S.mp4", OutputDir(storagePath, cameraID, t))
}
