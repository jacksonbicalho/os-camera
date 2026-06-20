package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"camera/frontend"
	"camera/internal/analysis"
	"camera/internal/config"
	"camera/internal/db"
	"camera/internal/exec"
	"camera/internal/ffprobe"
	"camera/internal/logger"
	"camera/internal/motion"
	"camera/internal/recorder"
	"camera/internal/server"
	"camera/internal/stateclass"
	"camera/internal/stateengine"
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
		Debug:      cfg.Debug,
		Output:     output,
		Path:       cfg.Log.Path,
		MaxSizeMB:  cfg.Log.MaxSizeMBOrDefault(),
		MaxAgeDays: cfg.Log.MaxAgeDaysOrDefault(),
		MaxBackups: cfg.Log.MaxBackupsOrDefault(),
		Compress:   cfg.Log.CompressOrDefault(),
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

	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("failed to create database directory %q: %v\n\nHint: run as root, or set db_path in camera.yaml to a user-writable path (e.g. db_path: ./camera.db)", dbDir, err)
	}
	if tmp, err := os.CreateTemp(dbDir, ".camera_write_check_*"); err != nil {
		log.Fatalf("database directory %q is not writable: %v\n\nHint: run as root, or set db_path in camera.yaml to a user-writable path (e.g. db_path: ./camera.db)", dbDir, err)
	} else {
		tmp.Close()
		os.Remove(tmp.Name())
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()
	database.WithLogger(slog)

	if database.IsNew {
		slog.Info("new database, seeding admin user from bootstrap config")
		if seedErr := db.SeedFromBootstrap(database, cfg); seedErr != nil {
			slog.Warn("seed from bootstrap failed", "error", seedErr)
		}
	}
	if err := db.EnsureStorageDefaults(database); err != nil {
		slog.Warn("ensure storage defaults failed", "error", err)
	}

	if n := storage.CleanOrphanedRecordings(database, slog); n > 0 {
		slog.Info("startup: orphaned recordings removed", "count", n)
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
		camMu          sync.Mutex
		cancelsByID    = make(map[string]context.CancelFunc)
		motionMonsByID = make(map[string]*motion.Monitor)
		streamsByID    = make(map[string]ffprobe.StreamInfo)
		wg             sync.WaitGroup
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

		camCtx, camCancel := context.WithCancel(ctx)
		reconnect := cam.EffectiveReconnectInterval()

		if cam.RecordingEnabled {
			rec := recorder.NewRecorder(cam, cfg.Storage, stream, commander, slog)
			wg.Add(1)
			go func() {
				defer wg.Done()
				rec.Run(camCtx, reconnect)
			}()
		}

		if cfg.Server.SegmentsPath != "" {
			str := streaming.NewHLSStreamer(cam, cfg.Server, stream, commander, slog)
			wg.Add(1)
			go func() {
				defer wg.Done()
				str.Run(camCtx, reconnect)
			}()
		}

		motionCfg := cam.EffectiveMotionConfig()
		if motionCfg.Enabled {
			camID := cam.ID
			mon := motion.New(cam, stream, motionCfg, cfg.Storage.Path, reconnect, slog,
				func() []zones.Zone {
					zs, _ := db.GetZones(database, camID)
					return zs
				},
				onMotionEvent)
			wg.Add(1)
			go func() {
				defer wg.Done()
				mon.Run(camCtx)
			}()

			camMu.Lock()
			motionMonsByID[cam.ID] = mon
			camMu.Unlock()

			if srv != nil {
				srv.WithMotionFeed(cam.ID, mon.Events())
				srv.WithRawFeed(cam.ID, mon.RawScores())
				srv.WithMonitor(cam.ID, mon)
			}
		}

		camMu.Lock()
		cancelsByID[cam.ID] = camCancel
		streamsByID[cam.ID] = stream
		camMu.Unlock()
	}

	stopCameraProcs := func(id string) {
		camMu.Lock()
		cancel := cancelsByID[id]
		delete(cancelsByID, id)
		delete(motionMonsByID, id)
		delete(streamsByID, id)
		camMu.Unlock()

		if cancel != nil {
			cancel()
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
			WithFrameExtractor(extractFrame).
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

		// Backfill device info for cameras registered before the feature
		// existed (best-effort, in the background so boot is not blocked).
		go srv.CaptureMissingDeviceInfo(context.Background())

		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		slog.Info("http server starting", "addr", addr)
		go func() {
			if err := http.ListenAndServe(addr, srv); err != nil {
				slog.Error("http server error", "error", err)
			}
		}()
		printStartupURLs(cfg.Server.Port)
	}

	// State classification: sobe um runner por intervalo para cada classificador
	// habilitado, se o serviço YOLO estiver configurado. O emit (SSE/notificação)
	// entra na S5; por ora só persiste o estado confirmado.
	// Renomeia (idempotente) pastas de classe antigas com espaços/acentos para slug.
	stateengine.MigrateSampleDirsToSlug(cfg.Storage.Path)
	if database != nil {
		if vacfg, err := db.GetVideoAnalysisConfig(database); err == nil && vacfg.ServiceURL != "" {
			rtspByID := make(map[string]string, len(cameras))
			for _, cam := range cameras {
				rtspByID[cam.ID] = cam.RTSPURL
			}
			deps := stateengine.Deps{
				Grabber:    stateengine.NewSnapshotGrabber(takeSnapshot, func(camID string) string { return rtspByID[camID] }, cfg.Storage.Path),
				Classifier: analysis.NewClient(vacfg.ServiceURL),
				Persist: func(cid int64, state string, conf float64, framePath string) error {
					return db.RecordStateTransition(database, cid, state, conf, framePath)
				},
				Emit: func(c stateclass.Classifier, state string, conf float64) {
					if srv != nil {
						srv.PublishClassifierState(c, state, conf)
					}
				},
				StoragePath: cfg.Storage.Path,
				Log:         slog,
			}
			var all []stateclass.Classifier
			for _, cam := range cameras {
				cs, err := db.ListStateClassifiers(database, cam.ID)
				if err != nil {
					continue
				}
				all = append(all, cs...)
			}
			if n := stateengine.StartRunners(context.Background(), all, deps); n > 0 {
				slog.Info("state classifiers running", "count", n)
			}
		}
	}

	storageCfg := db.StorageSettingsFromDB(database)
	cleanInterval := time.Duration(storageCfg.IntervalMinutes) * time.Minute
	if cleanInterval == 0 {
		cleanInterval = time.Hour
	}
	cleaner := storage.New(
		cfg.Storage.Path,
		storageCfg.WithMotionMinutes,
		storageCfg.WithoutMotionMinutes,
		config.DefaultChunkDuration,
		storageCfg.MaxSizeGB,
		storageCfg.WarnPercent,
		database,
		slog,
	)
	if srv != nil {
		srv.WithCleaner(cleaner)
	}
	go cleaner.Run(ctx, cleanInterval)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancel()

	slog.Info("shutting down, finalizing chunks...")
	wg.Wait()
	slog.Info("done")
}

func printStartupURLs(port int) {
	urls := []string{fmt.Sprintf("http://localhost:%d", port)}
	ifaces, err := net.InterfaceAddrs()
	if err == nil {
		seen := map[string]bool{}
		for _, addr := range ifaces {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				u := fmt.Sprintf("http://%s:%d", ip4, port)
				if !seen[u] {
					seen[u] = true
					urls = append(urls, u)
				}
			}
		}
	}
	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────┐")
	fmt.Println("│  Camera iniciado. Acesse pelo navegador:│")
	for _, u := range urls {
		fmt.Printf("│  %-39s│\n", u)
	}
	fmt.Println("└─────────────────────────────────────────┘")
	fmt.Println()
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

// extractFrame extrai um frame LIMPO (JPEG) de um MP4 no offset dado — a gravação
// não tem as anotações de movimento dos _motion.jpg. `-ss` antes do `-i` faz seek
// rápido por keyframe.
func extractFrame(ctx context.Context, path string, offsetSeconds float64) ([]byte, error) {
	cmd := osexec.CommandContext(ctx,
		"ffmpeg",
		"-ss", fmt.Sprintf("%.3f", offsetSeconds),
		"-i", path,
		"-frames:v", "1",
		"-f", "image2",
		"-vcodec", "mjpeg",
		"-",
	)
	return cmd.Output()
}
