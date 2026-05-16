package motion

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"testing"
)

// annotateFrame deve retornar um JPEG válido com as dimensões corretas.
func TestAnnotateFrameReturnsValidJPEG(t *testing.T) {
	w, h := 8, 6
	frame := make([]byte, w*h)
	bbox := BBox{X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
	data := annotateFrame(frame, w, h, bbox, 0.042, ColorGlobal)

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid JPEG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Errorf("expected %dx%d, got %dx%d", w, h, b.Dx(), b.Dy())
	}
}

// O retângulo anotado deve conter pixels brancos (255) na borda.
func TestAnnotateFrameRectangleBorderIsWhite(t *testing.T) {
	w, h := 16, 12
	frame := make([]byte, w*h) // frame totalmente preto
	bbox := BBox{X: 0.25, Y: 0.25, W: 0.5, H: 0.5}
	data := annotateFrame(frame, w, h, bbox, 0.1, ColorGlobal)

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid JPEG: %v", err)
	}

	// topo da borda do retângulo: y0 = round(0.25*12) = 3, x varia de x0..x1
	// Verificar que pelo menos um pixel na linha y=3 é claro (>200 após compressão JPEG)
	gray, ok := img.(*image.Gray)
	if !ok {
		// JPEG pode descomprimir como NRGBA; verificar luminância
		x0 := int(math.Round(bbox.X * float64(w)))
		y0 := int(math.Round(bbox.Y * float64(h)))
		r, g, b, _ := img.At(x0, y0).RGBA()
		lum := (r + g + b) / 3 >> 8
		if lum < 150 {
			t.Errorf("expected bright border pixel at (%d,%d), got lum=%d", x0, y0, lum)
		}
		return
	}
	x0 := int(math.Round(bbox.X * float64(w)))
	y0 := int(math.Round(bbox.Y * float64(h)))
	if gray.GrayAt(x0, y0).Y < 150 {
		t.Errorf("expected bright border pixel at (%d,%d)", x0, y0)
	}
}

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

// Frame vazio com bbox inteiro não deve entrar em pânico.
func TestAnnotateFrameFullBBox(t *testing.T) {
	w, h := 4, 4
	frame := make([]byte, w*h)
	bbox := BBox{X: 0, Y: 0, W: 1, H: 1}
	data := annotateFrame(frame, w, h, bbox, 1.0, ColorGlobal)
	if len(data) == 0 {
		t.Fatal("expected non-empty JPEG output")
	}
}
