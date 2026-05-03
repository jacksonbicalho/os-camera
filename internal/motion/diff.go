package motion

func diffFrames(a, b []byte) float64 {
	if len(a) != len(b) {
		panic("diffFrames: mismatched frame lengths")
	}
	if len(a) == 0 {
		return 0
	}
	var sum float64
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		sum += float64(d)
	}
	return sum / (float64(len(a)) * 255.0)
}
