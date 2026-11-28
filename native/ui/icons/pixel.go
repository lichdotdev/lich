package icons

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

//go:embed lich/*.png
var pixelFiles embed.FS

// PixelArt is a mark drawn on a pixel grid, black on clear, and drawn in the
// color it is given. It scales by whole multiples only, each pixel to a block:
// a smoothed or fractional scale would blur the grid it was drawn on, so on a
// fractional screen scale it is drawn a little smaller, centered.
type PixelArt struct {
	src    *image.NRGBA
	scaled map[bitmapKey]paint.ImageOp
}

var pixelArts = map[string]*PixelArt{}

// Lich is lich's own pixel mark of that name, decoded on first use. It
// panics on a name lich/ lacks, so a typo fails on the first frame that
// draws it.
func Lich(name string) *PixelArt {
	file := "lich/" + name + ".png"
	if a, ok := pixelArts[file]; ok {
		return a
	}
	data, err := pixelFiles.ReadFile(file)
	if err != nil {
		panic(fmt.Sprintf("icons: no pixel mark %q", name))
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("icons: %s: %v", file, err))
	}
	src := image.NewNRGBA(img.Bounds())
	draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)
	a := &PixelArt{src: src, scaled: map[bitmapKey]paint.ImageOp{}}
	pixelArts[file] = a
	return a
}

// Layout draws the mark in a size×size square, in c.
func (a *PixelArt) Layout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	k := bitmapKey{px: px, c: c}
	img, ok := a.scaled[k]
	if !ok {
		img = paint.NewImageOp(a.render(px, c))
		a.scaled[k] = img
	}
	img.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// render is the mark on a px square, each of its pixels a block of the
// largest whole size that fits, in c at the pixel's coverage.
func (a *PixelArt) render(px int, c color.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	w, h := a.src.Bounds().Dx(), a.src.Bounds().Dy()
	n := max(1, px/max(w, h))
	at := image.Pt((px-n*w)/2, (px-n*h)/2)
	for y := range h {
		for x := range w {
			cover := a.src.NRGBAAt(x, y).A
			if cover == 0 {
				continue
			}
			ink := c
			ink.A = uint8(uint16(cover) * uint16(c.A) / 255)
			block := image.Rect(x*n, y*n, (x+1)*n, (y+1)*n).Add(at)
			draw.Draw(dst, block, image.NewUniform(ink), image.Point{}, draw.Src)
		}
	}
	return dst
}
