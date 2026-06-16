// Package stateengine roda a inferência de estado de um classificador: a cada
// disparo (por enquanto, um ticker por intervalo), captura um recorte da câmera,
// chama o classificador (serviço YOLO), passa pela verificação de N consecutivos
// (stateclass.Tracker) e, na transição, persiste e emite o novo estado.
//
// Tudo via dependências injetadas (Grabber, StateClassifier, persist, emit) para
// ser testável sem ffmpeg/YOLO reais.
package stateengine

import (
	"context"
	"log/slog"
	"time"

	"camera/internal/analysis"
	"camera/internal/stateclass"
)

// Grabber captura um recorte (crop) da câmera e devolve o caminho de um arquivo
// de imagem acessível ao serviço YOLO, mais um cleanup para apagá-lo.
type Grabber interface {
	Grab(ctx context.Context, c stateclass.Classifier) (path string, cleanup func(), err error)
}

// Runner conduz um classificador.
type Runner struct {
	cfg        stateclass.Classifier
	grabber    Grabber
	classifier analysis.StateClassifier
	tracker    *stateclass.Tracker
	persist    func(classifierID int64, state string, confidence float64) error
	emit       func(c stateclass.Classifier, state string, confidence float64)
	log        *slog.Logger
}

func NewRunner(
	cfg stateclass.Classifier,
	grabber Grabber,
	classifier analysis.StateClassifier,
	persist func(int64, string, float64) error,
	emit func(stateclass.Classifier, string, float64),
	log *slog.Logger,
) *Runner {
	return &Runner{
		cfg:        cfg,
		grabber:    grabber,
		classifier: classifier,
		tracker:    stateclass.NewTracker(cfg.Threshold, cfg.MinConsecutive),
		persist:    persist,
		emit:       emit,
		log:        log,
	}
}

func topPrediction(preds []analysis.ClassPrediction) analysis.ClassPrediction {
	top := preds[0]
	for _, p := range preds[1:] {
		if p.Prob > top.Prob {
			top = p
		}
	}
	return top
}

// Step executa um ciclo: grab → classify → tracker → (na transição) persist + emit.
func (r *Runner) Step(ctx context.Context) error {
	path, cleanup, err := r.grabber.Grab(ctx, r.cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	preds, err := r.classifier.Classify(ctx, analysis.ClassifyRequest{Path: path, Model: r.cfg.Model})
	if err != nil {
		return err
	}
	if len(preds) == 0 {
		return nil
	}
	top := topPrediction(preds)
	changed, state := r.tracker.Observe(top.Label, top.Prob)
	if !changed {
		return nil
	}
	if err := r.persist(r.cfg.ID, state, top.Prob); err != nil {
		return err
	}
	if r.emit != nil {
		r.emit(r.cfg, state, top.Prob)
	}
	return nil
}

// Run roda Step a cada trigger_interval_seconds até ctx terminar. Retorna de
// imediato se o intervalo não for positivo.
func (r *Runner) Run(ctx context.Context) {
	interval := time.Duration(r.cfg.TriggerIntervalSeconds) * time.Second
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Step(ctx); err != nil && r.log != nil {
				r.log.Warn("state classifier step failed", "classifier", r.cfg.ID, "camera", r.cfg.CameraID, "error", err)
			}
		}
	}
}
