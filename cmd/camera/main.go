package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"camera/frontend"
	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/ffprobe"
	"camera/internal/logger"
	"camera/internal/motion"
	"camera/internal/recorder"
	"camera/internal/server"
	"camera/internal/storage"
	"camera/internal/streaming"
	"camera/internal/zones"
)

var version = "dev"
var commit = ""
var builtAt = ""

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			runInit(os.Args[2:])
			return
		case "version", "--version", "-v":
			fmt.Printf("camera %s (commit %s, built %s)\n", version, commit, builtAt)
			return
		}
	}

	configPath := flag.String("config", "camera.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	output := cfg.Log.Output
	if output == "" {
		output = "stdout"
	}

	slog, err := logger.New(logger.Options{
		Debug:  cfg.Debug,
		Output: output,
		Path:   cfg.Log.Path,
	})
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	if len(cfg.Cameras) == 0 {
		slog.Error("no cameras configured")
		os.Exit(1)
	}

	zonesPath := filepath.Join(cfg.Storage.Path, "motion_zones.json")
	zoneStore, err := zones.NewStore(zonesPath)
	if err != nil {
		log.Fatalf("failed to load zones: %v", err)
	}

	commander := exec.NewFFmpegCommander()
	prober := ffprobe.NewProber(&ffprobe.OSExecutor{})
	recorders := make([]*recorder.Recorder, 0, len(cfg.Cameras))
	streamers := make([]*streaming.HLSStreamer, 0, len(cfg.Cameras))
	streams := make(map[string]ffprobe.StreamInfo, len(cfg.Cameras))

	for _, cam := range cfg.Cameras {
		stream := resolveStream(cam, prober, slog)
		streams[cam.ID] = stream
		rec := recorder.NewRecorder(cam, cfg.Storage, stream, commander, slog)
		if err := rec.Start(time.Now().UTC()); err != nil {
			slog.Error("failed to start recorder", "camera", cam.ID, "error", err)
			os.Exit(1)
		}
		slog.Info("recording started", "camera", cam.ID)
		recorders = append(recorders, rec)

		if cfg.Server.SegmentsPath != "" {
			str := streaming.NewHLSStreamer(cam, cfg.Server, stream, commander, slog)
			if err := str.Start(); err != nil {
				slog.Error("failed to start hls streamer", "camera", cam.ID, "error", err)
				os.Exit(1)
			}
			streamers = append(streamers, str)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	if cfg.Server.Port > 0 {
		if cfg.Server.RecordingsPath == "" {
			cfg.Server.RecordingsPath = cfg.Storage.Path
		}
		static, err := fs.Sub(frontend.FS, "dist")
		if err != nil {
			log.Fatalf("failed to sub frontend fs: %v", err)
		}
		srv := server.NewServer(cfg.Server, cfg.Timezone, cfg.Cameras, slog, static).
			WithStorageConfig(cfg.Storage).
			WithVersion(version).
			WithBuildInfo(commit, builtAt).
			WithSystemConfig(cfg.Debug, cfg.Log).
			WithMotionConfig(cfg.Motion).
			WithZoneStore(zoneStore).
			WithSnapshotter(takeSnapshot)
		for id, si := range streams {
			srv.WithStreamInfo(id, si)
		}

		for _, cam := range cfg.Cameras {
			motionCfg := cam.EffectiveMotionConfig(cfg.Motion)
			if !motionCfg.Enabled {
				continue
			}
			stream := resolveStream(cam, prober, slog)
			camID := cam.ID
			mon := motion.New(cam, stream, motionCfg, cfg.Storage.Path, cam.EffectiveReconnectInterval(), slog,
				func() []zones.Zone { return zoneStore.Get(camID) })
			go mon.Run(ctx)
			srv.WithMotionFeed(cam.ID, mon.Events())
			srv.WithRawFeed(cam.ID, mon.RawScores())
		}

		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		slog.Info("http server starting", "addr", addr)
		go func() {
			if err := http.ListenAndServe(addr, srv); err != nil {
				slog.Error("http server error", "error", err)
			}
		}()
	} else {
		for _, cam := range cfg.Cameras {
			motionCfg := cam.EffectiveMotionConfig(cfg.Motion)
			if !motionCfg.Enabled {
				continue
			}
			stream := resolveStream(cam, prober, slog)
			camID := cam.ID
			mon := motion.New(cam, stream, motionCfg, cfg.Storage.Path, cam.EffectiveReconnectInterval(), slog,
				func() []zones.Zone { return zoneStore.Get(camID) })
			go mon.Run(ctx)
		}
	}

	cleanInterval := time.Duration(cfg.Storage.IntervalMinutes) * time.Minute
	if cleanInterval == 0 {
		cleanInterval = time.Hour
	}
	withMotion, withoutMotion := cfg.Storage.EffectiveRetention()
	cleaner := storage.New(
		cfg.Storage.Path,
		withMotion,
		withoutMotion,
		config.DefaultChunkDuration,
		cfg.Storage.MaxSizeGB,
		cfg.Storage.WarnPercent,
		slog,
	)
	go cleaner.Run(ctx, cleanInterval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()

	slog.Info("shutting down, finalizing chunks...")
	for _, str := range streamers {
		str.Stop()
	}
	for _, rec := range recorders {
		rec.Stop()
	}
	slog.Info("done")
}

func resolveStream(cam config.CameraConfig, prober *ffprobe.Prober, log *slog.Logger) ffprobe.StreamInfo {
	needsProbe := cam.VideoCodec == "" && cam.HasAudio == nil && cam.Width == 0 && cam.Height == 0
	var info ffprobe.StreamInfo
	if needsProbe {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		raw, err := prober.Probe(ctx, cam.RTSPURL)
		if err != nil {
			log.Warn("ffprobe failed, assuming audio present", "camera", cam.ID, "error", err)
			info.HasAudio = true
			return info
		}
		info, err = ffprobe.Parse(raw)
		if err != nil {
			log.Warn("ffprobe parse failed, assuming audio present", "camera", cam.ID, "error", err)
			info.HasAudio = true
			return info
		}
		log.Info("stream probed", "camera", cam.ID, "codec", info.VideoCodec,
			"has_audio", info.HasAudio, "width", info.Width, "height", info.Height)
	}
	if cam.VideoCodec != "" {
		info.VideoCodec = cam.VideoCodec
	}
	if cam.HasAudio != nil {
		info.HasAudio = *cam.HasAudio
	}
	if cam.Width != 0 {
		info.Width = cam.Width
	}
	if cam.Height != 0 {
		info.Height = cam.Height
	}
	return info
}

func takeSnapshot(ctx context.Context, rtspURL string) ([]byte, error) {
	cmd := osexec.CommandContext(ctx,
		"ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-frames:v", "1",
		"-f", "image2",
		"-vcodec", "mjpeg",
		"-",
	)
	return cmd.Output()
}
