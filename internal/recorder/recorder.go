package recorder

import (
	"fmt"
	"os"
	"time"

	"camera/internal/config"
)

type Process interface {
	Terminate() error
	Wait() error
}

type Commander interface {
	Start(name string, args ...string) (Process, error)
}

type Recorder struct {
	camera    config.CameraConfig
	storage   config.StorageConfig
	defaults  config.DefaultsConfig
	commander Commander
	process   Process
}

func NewRecorder(camera config.CameraConfig, storage config.StorageConfig, defaults config.DefaultsConfig, commander Commander) *Recorder {
	return &Recorder{
		camera:    camera,
		storage:   storage,
		defaults:  defaults,
		commander: commander,
	}
}

func (r *Recorder) Start(now time.Time) error {
	dir := OutputDir(r.storage.Path, r.camera.ID, now)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	pattern := OutputPattern(r.storage.Path, r.camera.ID, now)
	duration := int(r.camera.EffectiveChunkDuration(r.defaults).Seconds())
	proc, err := r.commander.Start("ffmpeg",
		"-i", r.camera.RTSPURL,
		"-f", "segment",
		"-segment_time", fmt.Sprintf("%d", duration),
		"-segment_format", "mp4",
		"-reset_timestamps", "1",
		"-strftime", "1",
		pattern,
	)
	if err != nil {
		return err
	}
	r.process = proc
	return nil
}

func (r *Recorder) Stop() {
	if r.process == nil {
		return
	}
	r.process.Terminate()
	r.process.Wait()
}

func OutputDir(storagePath, cameraID string, t time.Time) string {
	return fmt.Sprintf("%s/%s/%s", storagePath, cameraID, t.UTC().Format("2006/01/02"))
}

func OutputPattern(storagePath, cameraID string, t time.Time) string {
	return fmt.Sprintf("%s/%%Y%%m%%d%%H%%M%%S.mp4", OutputDir(storagePath, cameraID, t))
}
