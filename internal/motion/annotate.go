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

// annotateJPEGBytes decodes a color JPEG, draws the bbox rectangle and score
// label onto it with line thickness and text scale proportional to resolution,
// and re-encodes it.
func annotateJPEGBytes(data []byte, bbox BBox, score float64, c color.NRGBA, drawRect bool) ([]byte, error) {
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w := src.Bounds().Dx()
	h := src.Bounds().Dy()

	rgba := image.NewNRGBA(src.Bounds())
	draw.Draw(rgba, rgba.Bounds(), src, image.Point{}, draw.Src)

	thick := lineThickness(w, h)
	scale := textScale(w, h)

	if drawRect {
		x0 := clamp(int(math.Round(bbox.X*float64(w))), 0, w-1)
		y0 := clamp(int(math.Round(bbox.Y*float64(h))), 0, h-1)
		x1 := clamp(int(math.Round((bbox.X+bbox.W)*float64(w)))-1, x0, w-1)
		y1 := clamp(int(math.Round((bbox.Y+bbox.H)*float64(h)))-1, y0, h-1)
		drawHLineThick(rgba, x0, x1, y0, thick, c)
		drawHLineThick(rgba, x0, x1, y1, thick, c)
		drawVLineThick(rgba, y0, y1, x0, thick, c)
		drawVLineThick(rgba, y0, y1, x1, thick, c)
	}

	label := fmt.Sprintf("score: %.3f", score)
	labelW := len(label) * charWidth * scale
	const margin = 4
	tx := w - labelW - margin*scale
	if tx < 0 {
		tx = 0
	}
	ty := margin*scale + 13*scale
	shadow := color.NRGBA{A: 200}
	drawTextScaled(rgba, label, tx+scale, ty+scale, scale, shadow)
	drawTextScaled(rgba, label, tx, ty, scale, c)

	var buf bytes.Buffer
	jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: 85})
	return buf.Bytes(), nil
}

func lineThickness(w, h int) int {
	t := (w + h) / 500
	if t < 1 {
		t = 1
	}
	return t
}

func textScale(w, h int) int {
	s := h / 270
	if s < 1 {
		s = 1
	}
	return s
}

func drawHLineThick(img draw.Image, x0, x1, y, thickness int, c color.Color) {
	half := thickness / 2
	for dy := -half; dy < thickness-half; dy++ {
		drawHLine(img, x0, x1, y+dy, c)
	}
}

func drawVLineThick(img draw.Image, y0, y1, x, thickness int, c color.Color) {
	half := thickness / 2
	for dx := -half; dx < thickness-half; dx++ {
		drawVLine(img, y0, y1, x+dx, c)
	}
}

func drawTextScaled(img draw.Image, text string, x, y, scale int, c color.Color) {
	if scale <= 1 {
		drawText(img, text, x, y, c)
		return
	}
	textW := len(text) * charWidth
	const textH = 13
	tmp := image.NewNRGBA(image.Rect(0, 0, textW, textH))
	drawText(tmp, text, 0, 11, c)
	for ty := 0; ty < textH; ty++ {
		for tx := 0; tx < textW; tx++ {
			_, _, _, a := tmp.At(tx, ty).RGBA()
			if a > 0x8000 {
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						img.Set(x+tx*scale+sx, y+(ty-11)*scale+sy, c)
					}
				}
			}
		}
	}
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
