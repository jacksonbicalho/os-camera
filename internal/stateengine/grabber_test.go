package stateengine

import (
	"context"
	"image"
	"testing"

	"camera/internal/stateclass"
)

func TestCropNormalized(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	got := cropNormalized(img, 0.1, 0.2, 0.3, 0.4).Bounds()
	if got.Min.X != 10 || got.Min.Y != 20 || got.Dx() != 30 || got.Dy() != 40 {
		t.Fatalf("crop errado: %+v", got)
	}
}

func TestCropNormalizedClampsOverflow(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// x+w > 1 → clampa no limite direito
	got := cropNormalized(img, 0.8, 0.8, 0.5, 0.5).Bounds()
	if got.Max.X != 100 || got.Max.Y != 100 {
		t.Fatalf("clamp errado: %+v", got)
	}
	if got.Dx() <= 0 || got.Dy() <= 0 {
		t.Fatalf("crop vazio: %+v", got)
	}
}

func TestStartRunnersStartsOnlyEligible(t *testing.T) {
	cs := []stateclass.Classifier{
		{ID: 1, Enabled: true, TriggerIntervalSeconds: 1, Threshold: 0.8, MinConsecutive: 1},
		{ID: 2, Enabled: false, TriggerIntervalSeconds: 1},
		{ID: 3, Enabled: true, TriggerIntervalSeconds: 0},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // encerra as goroutines de imediato
	n := StartRunners(ctx, cs, Deps{})
	if n != 1 {
		t.Fatalf("esperava 1 runner elegível, got %d", n)
	}
}

func TestSelectIntervalRunners(t *testing.T) {
	cs := []stateclass.Classifier{
		{ID: 1, Enabled: true, TriggerIntervalSeconds: 10},  // ✓
		{ID: 2, Enabled: false, TriggerIntervalSeconds: 10}, // disabled
		{ID: 3, Enabled: true, TriggerIntervalSeconds: 0},   // sem intervalo
		{ID: 4, Enabled: true, TriggerIntervalSeconds: 5},   // ✓
	}
	got := SelectIntervalRunners(cs)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 4 {
		t.Fatalf("seleção errada: %+v", got)
	}
}
