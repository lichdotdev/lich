package icons

import (
	"image"
	"image/draw"

	"gioui.org/op/paint"
)

// Picture is img, an extension's logo, centered on a transparent square so a
// wide or tall one keeps its shape. It is drawn in its own colors, or, when
// every visible pixel is black, in the color it is drawn with, as a Lucide
// glyph is.
func Picture(img image.Image) *Bitmap {
	b := img.Bounds()
	side := max(b.Dx(), b.Dy())
	src := image.NewNRGBA(image.Rect(0, 0, side, side))
	at := image.Pt((side-b.Dx())/2, (side-b.Dy())/2)
	draw.Draw(src, image.Rectangle{Min: at, Max: at.Add(b.Size())}, img, b.Min, draw.Src)
	return &Bitmap{src: src, mono: inkOnly(src), scaled: map[bitmapKey]paint.ImageOp{}}
}
