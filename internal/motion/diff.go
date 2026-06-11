package motion

import (
	"math"

	"camera/internal/zones"
)

type BBox struct {
	X, Y, W, H float64
}

// bboxPixelThreshold é o delta mínimo (0–255) para considerar um pixel "diferente"
// na computação do bounding box do frame global. Mantido acima do ruído de I-frame
// de codecs H.264/H.265 (tipicamente 10–20 unidades).
const bboxPixelThreshold = 25

// zoneBboxPixelThreshold é o delta mínimo para bbox dentro de uma zona de detecção.
// Menor que bboxPixelThreshold porque: (a) dentro de uma zona o pior caso de um
// threshold baixo é cobrir a zona inteira — já o comportamento do fallback — e
// (b) zonas detectam movimento sutil (ex: gato à noite em câmera IR) onde as
// bordas do objeto têm diff de ~15–20 unidades.
const zoneBboxPixelThreshold = 12

// minBBoxDensity é a fração mínima de pixels acima do threshold dentro do bbox
// para considerá-lo válido. Filtra o caso em que pixels esparsos espalhados pelo
// frame (ruído de IR/JPEG/iluminação em texturas) inflam o bbox via min/max
// ingênuo. Objeto real tem densidade ≥ 5–25% no bbox; ruído esparso fica < 1%.
const minBBoxDensity = 0.03

// computeBBox retorna o bounding box normalizado (0.0–1.0) do maior componente
// conectado (8-conectividade) de pixels com diff acima do threshold em relação
// a prev. Pixels mascarados pelas zones são ignorados. Seleciona o componente
// com maior totalDiff, que corresponde ao objeto mais significativo (pessoa/gato)
// em vez de artefatos dispersos (sombras, reflexos de iluminação).
// Retorna (bbox, true) quando um componente foi encontrado, ou ({0,0,1,1}, false).
func computeBBox(prev, cur []byte, w, h int, zs []zones.Zone) (BBox, bool) {
	// Build per-pixel absolute diff (0 = below threshold or masked).
	diff := make([]int, len(prev))
	for i := range prev {
		if isMasked(i%w, i/w, w, h, zs) {
			continue
		}
		d := int(cur[i]) - int(prev[i])
		if d < 0 {
			d = -d
		}
		if d >= bboxPixelThreshold {
			diff[i] = d
		}
	}
	return largestDiffComponent(diff, w, h, minBBoxDensity, BBox{0, 0, 1, 1})
}

// computeBBoxInZone retorna o bounding box normalizado (0.0–1.0) dos pixels
// com maior diferença dentro da zona dz. Pixels fora da zona são ignorados.
// Retorna (bbox, true) quando pixels acima do threshold foram encontrados,
// ou (bbox da zona, false) quando nenhum pixel ultrapassou o threshold.
func computeBBoxInZone(prev, cur []byte, w, h int, dz zones.Zone) (BBox, bool) {
	x0z := int(math.Floor(dz.X * float64(w)))
	y0z := int(math.Floor(dz.Y * float64(h)))
	x1z := int(math.Ceil((dz.X + dz.W) * float64(w)))
	y1z := int(math.Ceil((dz.Y + dz.H) * float64(h)))

	diff := make([]int, len(prev))
	for y := y0z; y < y1z; y++ {
		for x := x0z; x < x1z; x++ {
			i := y*w + x
			d := int(cur[i]) - int(prev[i])
			if d < 0 {
				d = -d
			}
			if d >= zoneBboxPixelThreshold {
				diff[i] = d
			}
		}
	}

	fallback := BBox{dz.X, dz.Y, dz.W, dz.H}
	found, bbox := largestDiffComponentInRegion(diff, w, x0z, y0z, x1z, y1z, minBBoxDensity, fallback)
	return found, bbox
}

type bboxRegion struct {
	totalDiff, count       int
	minX, maxX, minY, maxY int
}

// largestDiffComponent finds the connected component (8-connectivity) with the
// highest totalDiff across the whole frame, then returns its normalised bbox.
func largestDiffComponent(diff []int, w, h int, minDensity float64, fallback BBox) (BBox, bool) {
	best, ok := findBestComponent(diff, w, 0, 0, w, h)
	if !ok {
		return fallback, false
	}
	bw := best.maxX - best.minX + 1
	bh := best.maxY - best.minY + 1
	if float64(best.count)/float64(bw*bh) < minDensity {
		return fallback, false
	}
	fw, fh := float64(w), float64(h)
	return BBox{
		X: float64(best.minX) / fw,
		Y: float64(best.minY) / fh,
		W: float64(bw) / fw,
		H: float64(bh) / fh,
	}, true
}

// largestDiffComponentInRegion is the zone-bounded variant of largestDiffComponent.
// Returns (bbox, found) — note argument order is swapped for zone fallback symmetry.
func largestDiffComponentInRegion(diff []int, w, x0, y0, x1, y1 int, minDensity float64, fallback BBox) (BBox, bool) {
	best, ok := findBestComponent(diff, w, x0, y0, x1, y1)
	if !ok {
		return fallback, false
	}
	bw := best.maxX - best.minX + 1
	bh := best.maxY - best.minY + 1
	if float64(best.count)/float64(bw*bh) < minDensity {
		return fallback, false
	}
	fw, fh := float64(w), float64(len(diff)/w)
	return BBox{
		X: float64(best.minX) / fw,
		Y: float64(best.minY) / fh,
		W: float64(bw) / fw,
		H: float64(bh) / fh,
	}, true
}

// findBestComponent runs BFS over diff[y*w+x] for pixels within [x0,x1)×[y0,y1),
// labels 8-connected components, and returns the one with the highest totalDiff.
func findBestComponent(diff []int, w, x0, y0, x1, y1 int) (bboxRegion, bool) {
	visited := make([]bool, len(diff))
	queue := make([]int, 0, 64)

	best := bboxRegion{}
	bestTotal := 0

	for sy := y0; sy < y1; sy++ {
		for sx := x0; sx < x1; sx++ {
			start := sy*w + sx
			if diff[start] == 0 || visited[start] {
				continue
			}
			visited[start] = true
			queue = append(queue[:0], start)
			r := bboxRegion{minX: x1, minY: y1, maxX: x0 - 1, maxY: y0 - 1}

			for head := 0; head < len(queue); head++ {
				idx := queue[head]
				px, py := idx%w, idx/w
				r.totalDiff += diff[idx]
				r.count++
				if px < r.minX {
					r.minX = px
				}
				if px > r.maxX {
					r.maxX = px
				}
				if py < r.minY {
					r.minY = py
				}
				if py > r.maxY {
					r.maxY = py
				}
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := px+dx, py+dy
						if nx < x0 || nx >= x1 || ny < y0 || ny >= y1 {
							continue
						}
						ni := ny*w + nx
						if diff[ni] > 0 && !visited[ni] {
							visited[ni] = true
							queue = append(queue, ni)
						}
					}
				}
			}

			if r.totalDiff > bestTotal {
				bestTotal = r.totalDiff
				best = r
			}
		}
	}

	return best, bestTotal > 0
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
			x1 := (x + 1) * w / newW
			y0 := y * h / newH
			y1 := (y + 1) * h / newH
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

// downscaleRGBToGray converts an RGB24 frame (srcW×srcH, 3 bytes/pixel) to a
// grayscale buffer at dstW×dstH using block-average downscale. When the source
// and destination dimensions match it only collapses RGB→gray. The luma weights
// match image/color.GrayModel (BT.601), so a frame whose channels are equal maps
// to that same value. This feeds the motion diff while the original RGB frame is
// kept full-resolution for the snapshot.
func downscaleRGBToGray(rgb []byte, srcW, srcH, dstW, dstH int) []byte {
	out := make([]byte, dstW*dstH)
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			x0 := x * srcW / dstW
			x1 := (x + 1) * srcW / dstW
			y0 := y * srcH / dstH
			y1 := (y + 1) * srcH / dstH
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var sum, count uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					i := (sy*srcW + sx) * 3
					r := uint32(rgb[i])
					g := uint32(rgb[i+1])
					b := uint32(rgb[i+2])
					y8 := (19595*r + 38470*g + 7471*b + 1<<15) >> 16
					sum += y8
					count++
				}
			}
			out[y*dstW+x] = byte(sum / count)
		}
	}
	return out
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

// updateBackground advances the background model toward cur by alpha (0–1).
// Call with a small alpha (~0.05) when no motion is detected so the background
// adapts slowly to lighting changes without absorbing moving objects.
func updateBackground(bg, cur []byte, alpha float64) {
	for i := range bg {
		bg[i] = byte(float64(bg[i])*(1-alpha) + float64(cur[i])*alpha + 0.5)
	}
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
