package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"camera/internal/config"
	"camera/internal/recorder"
)

func main() {
	configPath := flag.String("config", "camera.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	if len(cfg.Cameras) == 0 {
		log.Fatal("no cameras configured")
	}

	commander := recorder.NewFFmpegCommander()
	recorders := make([]*recorder.Recorder, 0, len(cfg.Cameras))

	for _, cam := range cfg.Cameras {
		rec := recorder.NewRecorder(cam, cfg.Storage, cfg.Defaults, commander)
		if err := rec.Start(time.Now().UTC()); err != nil {
			log.Fatalf("failed to start recorder for camera %s: %v", cam.ID, err)
		}
		log.Printf("recording started for camera %s", cam.ID)
		recorders = append(recorders, rec)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down, finalizing chunks...")
	for _, rec := range recorders {
		rec.Stop()
	}
	log.Println("done")
}
