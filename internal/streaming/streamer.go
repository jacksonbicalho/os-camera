package streaming

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"camera/internal/config"
	"camera/internal/exec"
)

type HLSStreamer struct {
	camera    config.CameraConfig
	server    config.ServerConfig
	commander exec.Commander
	log       *slog.Logger
	process   exec.Process
}

func NewHLSStreamer(camera config.CameraConfig, server config.ServerConfig, commander exec.Commander, log *slog.Logger) *HLSStreamer {
	return &HLSStreamer{
		camera:    camera,
		server:    server,
		commander: commander,
		log:       log,
	}
}

func (s *HLSStreamer) Start() error {
	dir := filepath.Join(s.server.SegmentsPath, s.camera.ID)
	s.log.Debug("creating segments directory", "path", dir, "camera", s.camera.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	playlist := filepath.Join(dir, "index.m3u8")
	segmentPattern := filepath.Join(dir, "%03d.ts")
	s.log.Debug("starting hls ffmpeg", "camera", s.camera.ID, "playlist", playlist)
	proc, err := s.commander.Start("ffmpeg",
		"-i", s.camera.RTSPURL,
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+append_list",
		"-hls_segment_filename", segmentPattern,
		playlist,
	)
	if err != nil {
		return fmt.Errorf("failed to start hls streamer for camera %s: %w", s.camera.ID, err)
	}
	s.process = proc
	s.log.Info("hls streaming started", "camera", s.camera.ID, "playlist", playlist)
	return nil
}

func (s *HLSStreamer) Stop() {
	if s.process == nil {
		return
	}
	s.log.Info("stopping hls streamer", "camera", s.camera.ID)
	s.process.Terminate()
	s.process.Wait()
}
