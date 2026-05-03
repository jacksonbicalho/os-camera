package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"camera/frontend"
	"camera/internal/config"
	"camera/internal/exec"
	"camera/internal/logger"
	"camera/internal/recorder"
	"camera/internal/server"
	"camera/internal/streaming"
)

func main() {
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

	commander := exec.NewFFmpegCommander()
	recorders := make([]*recorder.Recorder, 0, len(cfg.Cameras))
	streamers := make([]*streaming.HLSStreamer, 0, len(cfg.Cameras))

	for _, cam := range cfg.Cameras {
		rec := recorder.NewRecorder(cam, cfg.Storage, cfg.Defaults, commander, slog)
		if err := rec.Start(time.Now().UTC()); err != nil {
			slog.Error("failed to start recorder", "camera", cam.ID, "error", err)
			os.Exit(1)
		}
		slog.Info("recording started", "camera", cam.ID)
		recorders = append(recorders, rec)

		if cfg.Server.SegmentsPath != "" {
			str := streaming.NewHLSStreamer(cam, cfg.Server, commander, slog)
			if err := str.Start(); err != nil {
				slog.Error("failed to start hls streamer", "camera", cam.ID, "error", err)
				os.Exit(1)
			}
			streamers = append(streamers, str)
		}
	}

	if cfg.Server.Port > 0 {
		if cfg.Server.RecordingsPath == "" {
			cfg.Server.RecordingsPath = cfg.Storage.Path
		}
		static, err := fs.Sub(frontend.FS, "dist")
		if err != nil {
			log.Fatalf("failed to sub frontend fs: %v", err)
		}
		srv := server.NewServer(cfg.Server, cfg.Cameras, slog, static)
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		slog.Info("http server starting", "addr", addr)
		go func() {
			if err := http.ListenAndServe(addr, srv); err != nil {
				slog.Error("http server error", "error", err)
			}
		}()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down, finalizing chunks...")
	for _, str := range streamers {
		str.Stop()
	}
	for _, rec := range recorders {
		rec.Stop()
	}
	slog.Info("done")
}
