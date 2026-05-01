package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"camera/internal/config"
	"camera/internal/logger"
	"camera/internal/recorder"
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

	commander := recorder.NewFFmpegCommander()
	recorders := make([]*recorder.Recorder, 0, len(cfg.Cameras))

	for _, cam := range cfg.Cameras {
		rec := recorder.NewRecorder(cam, cfg.Storage, cfg.Defaults, commander, slog)
		if err := rec.Start(time.Now().UTC()); err != nil {
			slog.Error("failed to start recorder", "camera", cam.ID, "error", err)
			os.Exit(1)
		}
		slog.Info("recording started", "camera", cam.ID)
		recorders = append(recorders, rec)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down, finalizing chunks...")
	for _, rec := range recorders {
		rec.Stop()
	}
	slog.Info("done")
}
