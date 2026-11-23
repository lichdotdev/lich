package icons

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	xdraw "golang.org/x/image/draw"
)

//go:embed devicon/*.png material/*.png
var bitmapFiles embed.FS

// Bitmap is a multi-colored mark, kept as a picture rather than outlines: the
// devicons and Material's file icons draw with a color per shape, gradients,
// clip paths and evenodd fills the vector kit does not carry.
type Bitmap struct {
	src *image.NRGBA
	// mono marks ship with no fill of their own and take the color passed to
	// Layout.
	mono   bool
	scaled map[bitmapKey]paint.ImageOp
}

type bitmapKey struct {
	px int
	c  color.NRGBA
}

var bitmaps = map[string]*Bitmap{}

// bitmap is the PNG at file in bitmapFiles, decoded on first use. It panics
// on a file the set lacks, so a manifest out of step with its gen.sh fails on
// the first file of that kind.
func bitmap(file string) *Bitmap {
	if b, ok := bitmaps[file]; ok {
		return b
	}
	data, err := bitmapFiles.ReadFile(file)
	if err != nil {
		panic(fmt.Sprintf("icons: no bitmap %q", file))
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("icons: %s: %v", file, err))
	}
	src := image.NewNRGBA(img.Bounds())
	draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)
	b := &Bitmap{src: src, mono: inkOnly(src), scaled: map[bitmapKey]paint.ImageOp{}}
	bitmaps[file] = b
	return b
}

// inkBlack is the brightest channel a pixel may have and still count as the
// black a mark with no fill of its own renders in.
const inkBlack = 0x30

// inkOnly reports whether every visible pixel of img is black: a mark that
// set no fill, which on a dark ground would vanish unless recolored.
func inkOnly(img *image.NRGBA) bool {
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] != 0 && max(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) > inkBlack {
			return false
		}
	}
	return true
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

// LookupDevicon is the devicon of that name, as devicon.dev spells it ("go",
// "typescript", "css3"), or the PNG's own name ("GoOriginalWordmark"), for a name from
// outside lich: it reports a name the set lacks instead of panicking.
func LookupDevicon(name string) (*Bitmap, bool) {
	want := strings.ToLower(name)
	entries, err := bitmapFiles.ReadDir("devicon")
	if err != nil || want == "" {
		return nil, false
	}
	for _, e := range entries {
		stem, ok := strings.CutSuffix(e.Name(), ".png")
		if !ok || !deviconNamed(stem, want) {
			continue
		}
		return bitmap("devicon/" + e.Name()), true
	}
	return nil, false
}

// deviconNamed reports whether the PNG stem is the devicon want names, with
// or without the variant gen.sh appends.
func deviconNamed(stem, want string) bool {
	lower := strings.ToLower(stem)
	for _, variant := range []string{"originalwordmark", "original", "plain"} {
		if base, ok := strings.CutSuffix(lower, variant); ok && base == want {
			return true
		}
	}
	return lower == want
}
