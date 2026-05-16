package motion

import (
	"math"

	"camera/internal/zones"
)

type BBox struct {
	X, Y, W, H float64
}

// pixelThreshold é o delta mínimo (0–255) para considerar um pixel "diferente"
// na computação do bounding box.
const pixelThreshold = 30

// computeBBox retorna o bounding box normalizado (0.0–1.0) da região com maior
// diferença entre os frames. Pixels mascarados pelas zones são ignorados.
// Se nenhum pixel ultrapassar o threshold, retorna o frame inteiro {0,0,1,1}.
func computeBBox(prev, cur []byte, w, h int, zs []zones.Zone) BBox {
	minX, minY := w, h
	maxX, maxY := -1, -1
	for i := range prev {
		px := i % w
		py := i / w
		if isMasked(px, py, w, h, zs) {
			continue
		}
		d := int(cur[i]) - int(prev[i])
		if d < 0 {
			d = -d
		}
		if d >= pixelThreshold {
			if px < minX {
				minX = px
			}
			if py < minY {
				minY = py
			}
			if px > maxX {
				maxX = px
			}
			if py > maxY {
				maxY = py
			}
		}
	}
	if maxX < 0 {
		return BBox{0, 0, 1, 1}
	}
	fw, fh := float64(w), float64(h)
	return BBox{
		X: float64(minX) / fw,
		Y: float64(minY) / fh,
		W: float64(maxX-minX+1) / fw,
		H: float64(maxY-minY+1) / fh,
	}
}

func diffFrames(a, b []byte) float64 {
	return diffFramesMasked(a, b, len(a), 1, nil)
}

func diffFramesMasked(a, b []byte, w, h int, zs []zones.Zone) float64 {
	if len(a) != len(b) {
		panic("diffFrames: mismatched frame lengths")
	}
	if len(a) == 0 {
		return 0
	}
	var sum float64
	total := 0
	for i := range a {
		px := i % w
		py := i / w
		if isMasked(px, py, w, h, zs) {
			continue
		}
		total++
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		sum += float64(d)
	}
	if total == 0 {
		return 0
	}
	return sum / (float64(total) * 255.0)
}

// diffFramesForZone computes the mean absolute diff score for pixels INSIDE
// the given zone bounding box. Returns 0 if the zone has zero area.
func diffFramesForZone(a, b []byte, w, h int, z zones.Zone) float64 {
	if len(a) != len(b) {
		panic("diffFramesForZone: mismatched frame lengths")
	}
	if len(a) == 0 {
		return 0
	}
	x0 := int(math.Floor(z.X * float64(w)))
	y0 := int(math.Floor(z.Y * float64(h)))
	x1 := int(math.Ceil((z.X + z.W) * float64(w)))
	y1 := int(math.Ceil((z.Y + z.H) * float64(h)))
	var sum float64
	total := 0
	for i := range a {
		px := i % w
		py := i / w
		if px >= x0 && px < x1 && py >= y0 && py < y1 {
			total++
			d := int(a[i]) - int(b[i])
			if d < 0 {
				d = -d
			}
			sum += float64(d)
		}
	}
	if total == 0 {
		return 0
	}
	return sum / (float64(total) * 255.0)
}

// extractZone copia os pixels do recorte da zona para um buffer separado.
// Retorna o buffer e suas dimensões (zW × zH). Retorna nil se a zona for inválida.
func extractZone(frame []byte, frameW, frameH int, z zones.Zone) ([]byte, int, int) {
	x0 := int(math.Floor(z.X * float64(frameW)))
	y0 := int(math.Floor(z.Y * float64(frameH)))
	x1 := int(math.Ceil((z.X + z.W) * float64(frameW)))
	y1 := int(math.Ceil((z.Y + z.H) * float64(frameH)))
	zW := x1 - x0
	zH := y1 - y0
	if zW <= 0 || zH <= 0 {
		return nil, 0, 0
	}
	buf := make([]byte, zW*zH)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			buf[(y-y0)*zW+(x-x0)] = frame[y*frameW+x]
		}
	}
	return buf, zW, zH
}

// downsampleAvg reduz buf (w×h) pela escala dada usando média de blocos.
// scale ≤ 0 ou ≥ 1 retorna buf sem modificação.
func downsampleAvg(buf []byte, w, h int, scale float64) ([]byte, int, int) {
	if scale <= 0 || scale >= 1 {
		return buf, w, h
	}
	newW := max(1, int(math.Round(float64(w)*scale)))
	newH := max(1, int(math.Round(float64(h)*scale)))
	out := make([]byte, newW*newH)
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			x0 := x * w / newW
			x1 := (x+1) * w / newW
			y0 := y * h / newH
			y1 := (y+1) * h / newH
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var sum, count int
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					sum += int(buf[sy*w+sx])
					count++
				}
			}
			out[y*newW+x] = byte(sum / count)
		}
	}
	return out, newW, newH
}

// diffFramesForZoneScaled calcula o diff da zona com downscale opcional.
// Se z.Scale está entre 0 e 1 (exclusive), os pixels da zona são reduzidos por média
// antes da comparação. Scale=0 ou Scale=1 equivale a sem downscale.
func diffFramesForZoneScaled(a, b []byte, frameW, frameH int, z zones.Zone) float64 {
	za, zW, zH := extractZone(a, frameW, frameH, z)
	if zW == 0 || zH == 0 {
		return 0
	}
	zb, _, _ := extractZone(b, frameW, frameH, z)

	if z.Scale > 0 && z.Scale < 1 {
		origW, origH := zW, zH
		za, _, _ = downsampleAvg(za, origW, origH, z.Scale)
		zb, _, _ = downsampleAvg(zb, origW, origH, z.Scale)
	}

	return diffFrames(za, zb)
}

func isMasked(px, py, w, h int, zs []zones.Zone) bool {
	for _, z := range zs {
		x0 := int(math.Floor(z.X * float64(w)))
		y0 := int(math.Floor(z.Y * float64(h)))
		x1 := int(math.Ceil((z.X + z.W) * float64(w)))
		y1 := int(math.Ceil((z.Y + z.H) * float64(h)))
		if px >= x0 && px < x1 && py >= y0 && py < y1 {
			return true
		}
	}
	return false
}
