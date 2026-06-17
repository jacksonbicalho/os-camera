package stateengine

import (
	"image"

	"camera/internal/stateclass"
)

// cropNormalized recorta a região (x,y,w,h normalizados em 0–1) de img e devolve
// a sub-imagem correspondente, com clamp aos limites da imagem. É o miolo puro do
// Grabber (a captura do snapshot é integração).
func cropNormalized(img image.Image, x, y, w, h float64) image.Image {
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()

	x0 := b.Min.X + int(x*float64(iw))
	y0 := b.Min.Y + int(y*float64(ih))
	x1 := b.Min.X + int((x+w)*float64(iw))
	y1 := b.Min.Y + int((y+h)*float64(ih))

	x0 = clampInt(x0, b.Min.X, b.Max.X)
	y0 = clampInt(y0, b.Min.Y, b.Max.Y)
	x1 = clampInt(x1, x0+1, b.Max.X)
	y1 = clampInt(y1, y0+1, b.Max.Y)

	r := image.Rect(x0, y0, x1, y1)
	if si, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return si.SubImage(r)
	}
	// Fallback: cópia para tipos sem SubImage.
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for yy := r.Min.Y; yy < r.Max.Y; yy++ {
		for xx := r.Min.X; xx < r.Max.X; xx++ {
			dst.Set(xx-r.Min.X, yy-r.Min.Y, img.At(xx, yy))
		}
	}
	return dst
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// SelectIntervalRunners filtra os classificadores que devem ter um runner por
// intervalo: habilitados e com trigger_interval_seconds > 0.
func SelectIntervalRunners(cs []stateclass.Classifier) []stateclass.Classifier {
	out := []stateclass.Classifier{}
	for _, c := range cs {
		if c.Enabled && c.TriggerIntervalSeconds > 0 {
			out = append(out, c)
		}
	}
	return out
}
