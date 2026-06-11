package motion

import (
	"image/color"
	"testing"
)

func TestHexToNRGBA(t *testing.T) {
	tests := []struct {
		hex  string
		want color.NRGBA
	}{
		{"#ef4444", color.NRGBA{R: 239, G: 68, B: 68, A: 255}},
		{"#f97316", color.NRGBA{R: 249, G: 115, B: 22, A: 255}},
		{"ef4444", color.NRGBA{R: 239, G: 68, B: 68, A: 255}},
		{"#invalid", ColorGlobal},
		{"", ColorGlobal},
	}
	for _, tt := range tests {
		got := hexToNRGBA(tt.hex)
		if got != tt.want {
			t.Errorf("hexToNRGBA(%q) = %+v, want %+v", tt.hex, got, tt.want)
		}
	}
}

func TestLineThicknessProportional(t *testing.T) {
	// Low-res: 480×270 → thickness 1. High-res: 1920×1080 → thickness 6.
	if lineThickness(480, 270) != 1 {
		t.Errorf("expected thickness 1 at 480×270, got %d", lineThickness(480, 270))
	}
	if lineThickness(1920, 1080) != 6 {
		t.Errorf("expected thickness 6 at 1920×1080, got %d", lineThickness(1920, 1080))
	}
}

func TestTextScaleProportional(t *testing.T) {
	if textScale(480, 270) != 1 {
		t.Errorf("expected scale 1 at height 270, got %d", textScale(480, 270))
	}
	if textScale(1920, 1080) != 4 {
		t.Errorf("expected scale 4 at height 1080, got %d", textScale(1920, 1080))
	}
}
