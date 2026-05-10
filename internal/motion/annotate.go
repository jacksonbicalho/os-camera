package motion

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// charWidth é a largura de avanço de cada glyph em basicfont.Face7x13.
const charWidth = 7

// annotationColor é a cor usada para o retângulo e o texto do score.
var annotationColor = color.NRGBA{R: 255, G: 165, A: 255} // laranja

func annotateFrame(frame []byte, w, h int, bbox BBox, score float64) []byte {
	img := image.NewGray(image.Rect(0, 0, w, h))
	copy(img.Pix, frame)

	x0 := clamp(int(math.Round(bbox.X*float64(w))), 0, w-1)
	y0 := clamp(int(math.Round(bbox.Y*float64(h))), 0, h-1)
	x1 := clamp(int(math.Round((bbox.X+bbox.W)*float64(w)))-1, x0, w-1)
	y1 := clamp(int(math.Round((bbox.Y+bbox.H)*float64(h)))-1, y0, h-1)

	rgba := image.NewNRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	c := annotationColor
	for t := 0; t < 2; t++ {
		if y0+t < h {
			drawHLine(rgba, x0, x1, y0+t, c)
		}
		if y1-t >= 0 {
			drawHLine(rgba, x0, x1, y1-t, c)
		}
		if x0+t < w {
			drawVLine(rgba, y0, y1, x0+t, c)
		}
		if x1-t >= 0 {
			drawVLine(rgba, y0, y1, x1-t, c)
		}
	}

	label := fmt.Sprintf("score: %.3f", score)
	const margin = 4
	tx := w - len(label)*charWidth - margin
	if tx < 0 {
		tx = 0
	}
	drawText(rgba, label, tx, 11, c)

	var buf bytes.Buffer
	jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func drawHLine(img draw.Image, x0, x1, y int, c color.Color) {
	for x := x0; x <= x1; x++ {
		img.Set(x, y, c)
	}
}

func drawVLine(img draw.Image, y0, y1, x int, c color.Color) {
	for y := y0; y <= y1; y++ {
		img.Set(x, y, c)
	}
}

func drawText(img draw.Image, text string, x, y int, c color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}
