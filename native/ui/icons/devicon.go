package icons

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"path"
	"strings"
	"sync"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	xdraw "golang.org/x/image/draw"
)

//go:embed devicon/*.png devicon/devicons.json
var deviconFiles embed.FS

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

// deviconManifest is devicon/devicons.json: the icon for a file, by its
// lowercased base name first, then by its lowercased extension.
type deviconManifest struct {
	Extensions map[string]string `json:"extensions"`
	Names      map[string]string `json:"names"`
}

var devicons = sync.OnceValues(func() (deviconManifest, error) {
	var m deviconManifest
	data, err := deviconFiles.ReadFile("devicon/devicons.json")
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("devicons.json: %w", err)
	}
	return m, nil
})

var deviconBitmaps = map[string]*Bitmap{}

// DeviconFor returns the icon devicons.json gives the file at p, or nil when
// it gives none. It panics when the manifest names an icon with no PNG, so a
// manifest out of step with gen.sh fails on the first file of that kind.
func DeviconFor(p string) *Bitmap {
	m, err := devicons()
	if err != nil {
		panic(err)
	}
	base := strings.ToLower(path.Base(p))
	name, ok := m.Names[base]
	if !ok {
		name, ok = m.Extensions[extension(base)]
	}
	if !ok {
		return nil
	}
	if b, ok := deviconBitmaps[name]; ok {
		return b
	}
	b := loadDevicon(name)
	deviconBitmaps[name] = b
	return b
}

// extension is base's extension without the dot, "" for none or a dotfile.
func extension(base string) string {
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 {
		return ""
	}
	return base[dot+1:]
}

func loadDevicon(name string) *Bitmap {
	data, err := deviconFiles.ReadFile("devicon/" + name + ".png")
	if err != nil {
		panic(fmt.Sprintf("icons: no devicon %q", name))
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("icons: devicon %q: %v", name, err))
	}
	src := image.NewNRGBA(img.Bounds())
	draw.Draw(src, src.Bounds(), img, img.Bounds().Min, draw.Src)
	return &Bitmap{src: src, mono: inkOnly(src), scaled: map[bitmapKey]paint.ImageOp{}}
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
