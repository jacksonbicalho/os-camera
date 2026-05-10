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
