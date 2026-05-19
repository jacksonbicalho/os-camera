package recorder

import (
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
}

func NewRecorder(camera config.CameraConfig, storage config.StorageConfig, stream ffprobe.StreamInfo, commander exec.Commander, log *slog.Logger) *Recorder {
	return &Recorder{
		camera:    camera,
		storage:   storage,
		stream:    stream,
		commander: commander,
		log:       log,
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
		"-segment_format_options", "movflags=+frag_keyframe+empty_moov+default_base_moof",
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

func (r *Recorder) Stop() {
	if r.process == nil {
		return
	}
	r.log.Info("stopping recorder, finalizing chunk", "camera", r.camera.ID)
	r.process.Terminate()
	r.process.Wait()
}

func OutputDir(storagePath, cameraID string, t time.Time) string {
	return fmt.Sprintf("%s/%s/%s", storagePath, cameraID, t.UTC().Format("2006/01/02"))
}

func OutputPattern(storagePath, cameraID string, t time.Time) string {
	return fmt.Sprintf("%s/%%Y%%m%%d%%H%%M%%S.mp4", OutputDir(storagePath, cameraID, t))
}
