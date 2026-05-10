package motion

import (
	"math"

	"camera/internal/zones"
)

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
