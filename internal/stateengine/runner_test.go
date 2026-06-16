package stateengine_test

import (
	"context"
	"testing"

	"camera/internal/analysis"
	"camera/internal/stateclass"
	"camera/internal/stateengine"
)

type fakeGrabber struct{ grabs int }

func (g *fakeGrabber) Grab(context.Context, stateclass.Classifier) (string, func(), error) {
	g.grabs++
	return "/tmp/crop.jpg", func() {}, nil
}

// fakeClassifier devolve sempre a mesma predição de topo controlável.
type fakeClassifier struct {
	label string
	prob  float64
}

func (f *fakeClassifier) Classify(context.Context, analysis.ClassifyRequest) ([]analysis.ClassPrediction, error) {
	return []analysis.ClassPrediction{
		{Label: f.label, Prob: f.prob},
		{Label: "outro", Prob: 1 - f.prob},
	}, nil
}

func TestRunnerStepEmitsOnlyOnConfirmedTransition(t *testing.T) {
	cfg := stateclass.Classifier{ID: 7, CameraID: "cam1", Threshold: 0.8, MinConsecutive: 3, Classes: []string{"aberto", "fechado"}}
	cls := &fakeClassifier{label: "aberto", prob: 0.95}

	var persisted []string
	var emitted []string
	persist := func(id int64, state string, conf float64) error {
		if id != 7 {
			t.Fatalf("classifier id errado: %d", id)
		}
		persisted = append(persisted, state)
		return nil
	}
	emit := func(_ stateclass.Classifier, state string, _ float64) {
		emitted = append(emitted, state)
	}

	r := stateengine.NewRunner(cfg, &fakeGrabber{}, cls, persist, emit, nil)
	ctx := context.Background()

	// 2 ciclos: ainda sem confirmação (N=3)
	r.Step(ctx)
	r.Step(ctx)
	if len(persisted) != 0 || len(emitted) != 0 {
		t.Fatalf("não deveria persistir/emitir antes de N leituras, got %v / %v", persisted, emitted)
	}
	// 3º ciclo: confirma → persiste + emite uma vez
	r.Step(ctx)
	if len(persisted) != 1 || persisted[0] != "aberto" || len(emitted) != 1 {
		t.Fatalf("esperava 1 persist+emit 'aberto', got %v / %v", persisted, emitted)
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
		func(int64, string, float64) error { persisted++; return nil }, nil, nil)
	for i := 0; i < 5; i++ {
		r.Step(context.Background())
	}
	if persisted != 0 {
		t.Fatalf("leituras abaixo do limiar não deveriam persistir, got %d", persisted)
	}
}
