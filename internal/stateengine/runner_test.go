package stateengine_test

import (
	"context"
	"testing"
	"time"

	"camera/internal/analysis"
	"camera/internal/stateclass"
	"camera/internal/stateengine"
)

type fakeGrabber struct{ grabs int }

func (g *fakeGrabber) Grab(context.Context, stateclass.Classifier) (string, func(), error) {
	g.grabs++
	return "/tmp/crop.jpg", func() {}, nil
}

// fakeClassifier devolve sempre a mesma predição de topo controlável e guarda o
// último modelo pedido (para checar que cada classificador usa o seu).
type fakeClassifier struct {
	label     string
	prob      float64
	lastModel string
}

func (f *fakeClassifier) Classify(_ context.Context, req analysis.ClassifyRequest) ([]analysis.ClassPrediction, error) {
	f.lastModel = req.Model
	return []analysis.ClassPrediction{
		{Label: f.label, Prob: f.prob},
		{Label: "outro", Prob: 1 - f.prob},
	}, nil
}

func TestRunnerStepEmitsOnlyOnConfirmedTransition(t *testing.T) {
	cfg := stateclass.Classifier{ID: 7, CameraID: "cam1", Threshold: 0.8, MinConsecutive: 3, Classes: []string{"aberto", "fechado"}}
	cls := &fakeClassifier{label: "aberto", prob: 0.95}

	var persisted []string
	var persistedFrames []string
	var emitted []string
	var savedSrc []string
	persist := func(id int64, state string, conf float64, framePath string) error {
		if id != 7 {
			t.Fatalf("classifier id errado: %d", id)
		}
		persisted = append(persisted, state)
		persistedFrames = append(persistedFrames, framePath)
		return nil
	}
	emit := func(_ stateclass.Classifier, state string, _ float64) {
		emitted = append(emitted, state)
	}
	saveFrame := func(srcPath string, cid int64, _ time.Time) (string, error) {
		savedSrc = append(savedSrc, srcPath)
		return "/recordings/state_history/7/123.jpg", nil
	}

	r := stateengine.NewRunner(cfg, &fakeGrabber{}, cls, persist, emit, saveFrame, nil)
	ctx := context.Background()

	// 2 ciclos: ainda sem confirmação (N=3)
	r.Step(ctx)
	r.Step(ctx)
	if len(persisted) != 0 || len(emitted) != 0 {
		t.Fatalf("não deveria persistir/emitir antes de N leituras, got %v / %v", persisted, emitted)
	}
	// 3º ciclo: confirma → persiste + emite uma vez, com o thumb da transição
	r.Step(ctx)
	if len(persisted) != 1 || persisted[0] != "aberto" || len(emitted) != 1 {
		t.Fatalf("esperava 1 persist+emit 'aberto', got %v / %v", persisted, emitted)
	}
	if len(savedSrc) != 1 || savedSrc[0] != "/tmp/crop.jpg" {
		t.Fatalf("saveFrame deveria receber o crop grabbado, got %v", savedSrc)
	}
	if persistedFrames[0] != "/recordings/state_history/7/123.jpg" {
		t.Fatalf("framePath do saveFrame deveria ir ao persist, got %q", persistedFrames[0])
	}
	// usa o modelo DO PRÓPRIO classificador (id 7), não o compartilhado
	if cls.lastModel != "custom-cls-7" {
		t.Fatalf("esperava modelo custom-cls-7, got %q", cls.lastModel)
	}
	// 4º ciclo (mesmo estado): nada novo
	r.Step(ctx)
	if len(persisted) != 1 || len(emitted) != 1 {
		t.Fatalf("não deveria re-emitir estado estável, got %v / %v", persisted, emitted)
	}
}

func TestRunnerStepBelowThresholdDoesNothing(t *testing.T) {
	cfg := stateclass.Classifier{ID: 1, Threshold: 0.8, MinConsecutive: 1}
	cls := &fakeClassifier{label: "aberto", prob: 0.5} // abaixo do limiar
	var persisted int
	r := stateengine.NewRunner(cfg, &fakeGrabber{}, cls,
		func(int64, string, float64, string) error { persisted++; return nil }, nil, nil, nil)
	for i := 0; i < 5; i++ {
		r.Step(context.Background())
	}
	if persisted != 0 {
		t.Fatalf("leituras abaixo do limiar não deveriam persistir, got %d", persisted)
	}
}
