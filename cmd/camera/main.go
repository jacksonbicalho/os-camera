package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
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

	onMotionEvent := func(cameraID string, t time.Time, score float64, frame, label, color string, bbox motion.BBox) {
		ev := db.MotionEvent{
			CameraID:   cameraID,
			OccurredAt: t,
			Score:      score,
			FramePath:  frame,
			Label:      label,
			Color:      color,
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
		stream := ffprobe.Resolve(context.Background(), ffprobe.Resolver{
			VideoCodec: cam.VideoCodec,
			HasAudio:   cam.HasAudio,
			Width:      cam.Width,
			Height:     cam.Height,
			RTSPURL:    cam.RTSPURL,
		}, prober, slog)

		// Persiste os dados detectados pelo ffprobe no banco.
		// Só atualiza quando ffprobe detectou dimensões reais (Width > 0) e a câmera
		// ainda não tem dimensões salvas — evita gravar fallbacks de falha no banco.
		if database != nil && stream.Width > 0 && cam.Width == 0 {
			if err := db.UpdateCameraStreamInfo(database, cam.ID, stream.VideoCodec, &stream.HasAudio, stream.Width, stream.Height); err != nil {
				slog.Warn("failed to persist stream info", "camera", cam.ID, "error", err)
			}
		}

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
				func() []zones.Zone {
					zs, _ := db.GetZones(database, camID)
					return zs
				},
				onMotionEvent)
			go mon.Run(motionCtx)

			camMu.Lock()
			motionCancelsByID[cam.ID] = motionCancel
			motionMonsByID[cam.ID] = mon
			camMu.Unlock()

			if srv != nil {
				srv.WithMotionFeed(cam.ID, mon.Events())
				srv.WithRawFeed(cam.ID, mon.RawScores())
				srv.WithMonitor(cam.ID, mon)
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
			WithSnapshotter(takeSnapshot).
			WithCameraCallbacks(startCameraProcs, stopCameraProcs).
			WithDB(database).
			WithProber(prober)

		camMu.Lock()
		for id, si := range streamsByID {
			srv.WithStreamInfo(id, si)
		}
		for id, mon := range motionMonsByID {
			srv.WithMotionFeed(id, mon.Events())
			srv.WithRawFeed(id, mon.RawScores())
			srv.WithMonitor(id, mon)
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
