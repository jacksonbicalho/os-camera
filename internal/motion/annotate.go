package motion

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// charWidth é a largura de avanço de cada glyph em basicfont.Face7x13.
const charWidth = 7

var (
	// ColorGlobal é a cor para detecção global de movimento.
	ColorGlobal = color.NRGBA{R: 255, G: 165, A: 255}
	// ColorDetect é a cor para zonas de detecção (Tailwind orange-500).
	ColorDetect = color.NRGBA{R: 249, G: 115, B: 22, A: 255}
)

func annotateFrame(frame []byte, w, h int, bbox BBox, score float64, c color.NRGBA, drawRect bool) []byte {
	img := image.NewGray(image.Rect(0, 0, w, h))
	copy(img.Pix, frame)

	rgba := image.NewNRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	if drawRect {
		x0 := clamp(int(math.Round(bbox.X*float64(w))), 0, w-1)
		y0 := clamp(int(math.Round(bbox.Y*float64(h))), 0, h-1)
		x1 := clamp(int(math.Round((bbox.X+bbox.W)*float64(w)))-1, x0, w-1)
		y1 := clamp(int(math.Round((bbox.Y+bbox.H)*float64(h)))-1, y0, h-1)
		drawHLine(rgba, x0, x1, y0, c)
		drawHLine(rgba, x0, x1, y1, c)
		drawVLine(rgba, y0, y1, x0, c)
		drawVLine(rgba, y0, y1, x1, c)
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

// hexToNRGBA converte uma cor hexadecimal (com ou sem "#") em color.NRGBA.
// Retorna ColorGlobal como fallback para entradas inválidas.
func hexToNRGBA(hex string) color.NRGBA {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return ColorGlobal
	}
	r, err1 := strconv.ParseUint(h[0:2], 16, 8)
	g, err2 := strconv.ParseUint(h[2:4], 16, 8)
	b, err3 := strconv.ParseUint(h[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return ColorGlobal
	}
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}
