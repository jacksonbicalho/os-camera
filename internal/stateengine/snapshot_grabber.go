package stateengine

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // registra o decoder PNG (snapshots normalmente são JPEG)
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"camera/internal/analysis"
	"camera/internal/stateclass"
)

// SnapFunc captura um JPEG a partir de uma URL RTSP (é o snapFn que o server já usa).
type SnapFunc func(ctx context.Context, rtsp string) ([]byte, error)

// SnapshotGrabber implementa Grabber: snapshot via SnapFunc → crop da região →
// grava o JPEG sob storagePath/tmp. Como o serviço YOLO monta o mesmo `./storage`
// em `/data` que a câmera, o path gravado é o mesmo que o `/classify` lê — sem
// tradução (igual ao que o `/analyze` já faz com as gravações).
type SnapshotGrabber struct {
	snap        SnapFunc
	rtspOf      func(cameraID string) string
	storagePath string
}

func NewSnapshotGrabber(snap SnapFunc, rtspOf func(string) string, storagePath string) *SnapshotGrabber {
	return &SnapshotGrabber{snap: snap, rtspOf: rtspOf, storagePath: storagePath}
}

func (g *SnapshotGrabber) Grab(ctx context.Context, c stateclass.Classifier) (string, func(), error) {
	data, err := g.snap(ctx, g.rtspOf(c.CameraID))
	if err != nil {
		return "", nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("decode snapshot: %w", err)
	}
	crop := cropNormalized(img, c.CropX, c.CropY, c.CropW, c.CropH)

	dir := filepath.Join(g.storagePath, "tmp")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("classify_%d_%d.jpg", c.ID, time.Now().UnixNano()))
	f, err := os.Create(path)
	if err != nil {
		return "", nil, err
	}
	if err := jpeg.Encode(f, crop, &jpeg.Options{Quality: 90}); err != nil {
		f.Close()
		os.Remove(path)
		return "", nil, err
	}
	f.Close()
	return path, func() { os.Remove(path) }, nil
}

// Deps reúne as dependências para subir os runners de classificação de estado.
type Deps struct {
	Grabber    Grabber
	Classifier analysis.StateClassifier
	Persist    func(classifierID int64, state string, confidence float64) error
	Emit       func(c stateclass.Classifier, state string, confidence float64)
	Log        *slog.Logger
}

// StartRunners sobe, em goroutines, um Runner por classificador elegível
// (habilitado + intervalo > 0) e retorna quantos foram iniciados.
func StartRunners(ctx context.Context, classifiers []stateclass.Classifier, d Deps) int {
	sel := SelectIntervalRunners(classifiers)
	for _, c := range sel {
		r := NewRunner(c, d.Grabber, d.Classifier, d.Persist, d.Emit, d.Log)
		go r.Run(ctx)
	}
	return len(sel)
}
