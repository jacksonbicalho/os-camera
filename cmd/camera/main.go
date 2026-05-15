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
	"sync"
	"syscall"
	"time"

	"camera/frontend"
	"camera/internal/config"
	"camera/internal/db"
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

	dbPath := cfg.DBPath
	if dbPath == "" {
		dbDir := cfg.Storage.Path
		if dbDir == "" {
			dbDir = "."
		}
		dbPath = filepath.Join(dbDir, "camera.db")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("failed to create database directory: %v", err)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if database.IsNew {
		slog.Info("new database, seeding admin user from bootstrap config")
		if seedErr := db.SeedFromBootstrap(database, cfg); seedErr != nil {
			slog.Warn("seed from bootstrap failed", "error", seedErr)
		}
	}

	cameras, err := db.ListCameras(database)
	if err != nil {
		log.Fatalf("failed to load cameras from database: %v", err)
	}
	slog.Info("cameras loaded from database", "count", len(cameras))

	zonesPath := filepath.Join(cfg.Storage.Path, "motion_zones.json")
	zoneStore, err := zones.NewStore(zonesPath)
	if err != nil {
		log.Fatalf("failed to load zones: %v", err)
	}

	commander := exec.NewFFmpegCommander()
	prober := ffprobe.NewProber(&ffprobe.OSExecutor{})

	ctx, cancel := context.WithCancel(context.Background())

	var (
		camMu             sync.Mutex
		recordersByID     = make(map[string]*recorder.Recorder)
		streamersByID     = make(map[string]*streaming.HLSStreamer)
		motionCancelsByID = make(map[string]context.CancelFunc)
		motionMonsByID    = make(map[string]*motion.Monitor)
		streamsByID       = make(map[string]ffprobe.StreamInfo)
	)

	// srv is assigned after NewServer; callbacks close over this variable.
	var srv *server.Server

	onMotionEvent := func(cameraID string, t time.Time, score float64, frame string, bbox motion.BBox) {
		ev := db.MotionEvent{
			CameraID:   cameraID,
			OccurredAt: t,
			Score:      score,
			FramePath:  frame,
			BboxX:      bbox.X,
			BboxY:      bbox.Y,
			BboxW:      bbox.W,
			BboxH:      bbox.H,
		}
		if err := db.InsertMotionEvent(database, ev); err != nil {
			slog.Warn("failed to record motion event in DB", "camera", cameraID, "error", err)
		}
		if err := db.MarkRecordingHasMotion(database, cameraID, t, t.Add(time.Second)); err != nil {
			slog.Warn("failed to mark recording has_motion", "camera", cameraID, "error", err)
		}
	}

	startCameraProcs := func(cam config.CameraConfig) {
		stream := resolveStream(cam, prober, slog)

		rec := recorder.NewRecorder(cam, cfg.Storage, stream, commander, slog)
		if err := rec.Start(time.Now().UTC()); err != nil {
			slog.Error("failed to start recorder", "camera", cam.ID, "error", err)
			return
		}
		slog.Info("recording started", "camera", cam.ID)

		camMu.Lock()
		recordersByID[cam.ID] = rec
		streamsByID[cam.ID] = stream
		camMu.Unlock()

		if cfg.Server.SegmentsPath != "" {
			str := streaming.NewHLSStreamer(cam, cfg.Server, stream, commander, slog)
			if err := str.Start(); err != nil {
				slog.Error("failed to start hls streamer", "camera", cam.ID, "error", err)
			} else {
				camMu.Lock()
				streamersByID[cam.ID] = str
				camMu.Unlock()
			}
		}

		motionCfg := cam.EffectiveMotionConfig()
		if motionCfg.Enabled {
			camID := cam.ID
			motionCtx, motionCancel := context.WithCancel(ctx)
			mon := motion.New(cam, stream, motionCfg, cfg.Storage.Path, cam.EffectiveReconnectInterval(), slog,
				func() []zones.Zone { return zoneStore.Get(camID) },
				onMotionEvent)
			go mon.Run(motionCtx)

			camMu.Lock()
			motionCancelsByID[cam.ID] = motionCancel
			motionMonsByID[cam.ID] = mon
			camMu.Unlock()

			if srv != nil {
				srv.WithMotionFeed(cam.ID, mon.Events())
				srv.WithRawFeed(cam.ID, mon.RawScores())
			}
		}
	}

	stopCameraProcs := func(id string) {
		camMu.Lock()
		rec := recordersByID[id]
		str := streamersByID[id]
		motionCancel := motionCancelsByID[id]
		delete(recordersByID, id)
		delete(streamersByID, id)
		delete(motionCancelsByID, id)
		delete(motionMonsByID, id)
		delete(streamsByID, id)
		camMu.Unlock()

		if rec != nil {
			rec.Stop()
		}
		if str != nil {
			str.Stop()
		}
		if motionCancel != nil {
			motionCancel()
		}
	}

	for _, cam := range cameras {
		startCameraProcs(cam)
	}

	if cfg.Server.Port > 0 {
		if cfg.Server.RecordingsPath == "" {
			cfg.Server.RecordingsPath = cfg.Storage.Path
		}
		static, err := fs.Sub(frontend.FS, "dist")
		if err != nil {
			log.Fatalf("failed to sub frontend fs: %v", err)
		}
		srv = server.NewServer(cfg.Server, cfg.Timezone, cameras, slog, static).
			WithStorageConfig(cfg.Storage).
			WithVersion(version).
			WithBuildInfo(commit, builtAt).
			WithSystemConfig(cfg.Debug, cfg.Log).
			WithZoneStore(zoneStore).
			WithSnapshotter(takeSnapshot).
			WithCameraCallbacks(startCameraProcs, stopCameraProcs).
			WithDB(database)

		camMu.Lock()
		for id, si := range streamsByID {
			srv.WithStreamInfo(id, si)
		}
		for id, mon := range motionMonsByID {
			srv.WithMotionFeed(id, mon.Events())
			srv.WithRawFeed(id, mon.RawScores())
		}
		camMu.Unlock()

		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		slog.Info("http server starting", "addr", addr)
		go func() {
			if err := http.ListenAndServe(addr, srv); err != nil {
				slog.Error("http server error", "error", err)
			}
		}()
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
		database,
		slog,
	)
	go cleaner.Run(ctx, cleanInterval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()

	slog.Info("shutting down, finalizing chunks...")

	camMu.Lock()
	strs := make([]*streaming.HLSStreamer, 0, len(streamersByID))
	for _, str := range streamersByID {
		strs = append(strs, str)
	}
	recs := make([]*recorder.Recorder, 0, len(recordersByID))
	for _, rec := range recordersByID {
		recs = append(recs, rec)
	}
	camMu.Unlock()

	for _, str := range strs {
		str.Stop()
	}
	for _, rec := range recs {
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
