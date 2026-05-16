package streaming

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/ffprobe"
)

type HLSStreamer struct {
	camera    config.CameraConfig
	server    config.ServerConfig
	stream    ffprobe.StreamInfo
	commander exec.Commander
	log       *slog.Logger
	process   exec.Process
}

func NewHLSStreamer(camera config.CameraConfig, server config.ServerConfig, stream ffprobe.StreamInfo, commander exec.Commander, log *slog.Logger) *HLSStreamer {
	return &HLSStreamer{
		camera:    camera,
		server:    server,
		stream:    stream,
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
	segmentPattern := filepath.Join(dir, "%06d.ts")
	s.log.Debug("starting hls ffmpeg", "camera", s.camera.ID, "playlist", playlist)
	args := []string{"-i", s.camera.RTSPURL}
	if s.needsTranscode() {
		s.log.Warn("transcoding video to h264", "camera", s.camera.ID, "source_codec", s.stream.VideoCodec, "mode", s.camera.HLSVideoMode)
		args = append(args, "-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency")
		if s.stream.HasAudio {
			args = append(args, "-c:a", "copy")
		} else {
			args = append(args, "-an")
		}
	} else {
		args = append(args, "-c", "copy")
		if !s.stream.HasAudio {
			args = append(args, "-an")
		}
	}

	const segmentSeconds = 2
	listSize, hlsFlags := hlsListSizeAndFlags(s.server.HLSDVRSeconds, segmentSeconds)
	args = append(args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentSeconds),
		"-hls_list_size", strconv.Itoa(listSize),
		"-hls_flags", hlsFlags,
		"-hls_segment_filename", segmentPattern,
		playlist,
	)
	proc, err := s.commander.Start("ffmpeg", args...)
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

func (s *HLSStreamer) needsTranscode() bool {
	switch s.camera.HLSVideoMode {
	case "h264":
		return true
	case "copy":
		return false
	default: // "auto" or empty
		return s.stream.VideoCodec != "" && s.stream.VideoCodec != "h264"
	}
}

func hlsListSizeAndFlags(dvrSeconds, segmentSeconds int) (listSize int, flags string) {
	if dvrSeconds <= 0 {
		return 5, "delete_segments+append_list+independent_segments"
	}
	size := dvrSeconds / segmentSeconds
	if size < 5 {
		size = 5
	}
	parts := []string{"append_list", "independent_segments", "program_date_time"}
	return size, strings.Join(parts, "+")
}
