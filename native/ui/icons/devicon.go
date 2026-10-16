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
	xdraw "golang.org/x/image/draw"
)

//go:embed devicon/*.png
var deviconPNGs embed.FS

// Bitmap is a multi-colored mark, kept as a picture rather than outlines: the
// devicons draw with gradients, clip paths and evenodd fills the vector kit
// does not carry.
type Bitmap struct {
	src *image.NRGBA
	// mono marks ship with no fill of their own and take the color passed to
	// Layout, as FileIcon.tsx draws them with fill=currentColor.
	mono   bool
	scaled map[bitmapKey]paint.ImageOp
}

type bitmapKey struct {
	px int
	c  color.NRGBA
}

// Devicon returns the devicon of that name (the file under devicon/, "go",
// "typescript"). mono draws it in Layout's color instead of its own. It panics
// on a name the set does not have, like Lucide.
func Devicon(name string, mono bool) *Bitmap {
	data, err := deviconPNGs.ReadFile("devicon/" + name + ".png")
	if err != nil {
		panic(fmt.Sprintf("icons: no devicon %q", name))
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("icons: devicon %q: %v", name, err))
	}
	src := image.NewNRGBA(img.Bounds())
	draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)
	return &Bitmap{src: src, mono: mono, scaled: map[bitmapKey]paint.ImageOp{}}
}

// Layout draws the mark in a size×size square; c is used only by a mono mark.
func (b *Bitmap) Layout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	k := bitmapKey{px: px}
	if b.mono {
		k.c = c
	}
	img, ok := b.scaled[k]
	if !ok {
		img = paint.NewImageOp(b.render(px, k.c))
		b.scaled[k] = img
	}
	img.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// render scales the mark to px and, for a mono mark, recolors every pixel to
// c, keeping its coverage.
func (b *Bitmap) render(px int, c color.NRGBA) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, px, px))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), b.src, b.src.Bounds(), xdraw.Src, nil)
	if !b.mono {
		return dst
	}
	for i := 0; i < len(dst.Pix); i += 4 {
		a := uint16(dst.Pix[i+3]) * uint16(c.A) / 255
		dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = c.R, c.G, c.B, uint8(a)
	}
	return dst
}
