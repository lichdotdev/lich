// Package glyphs rasterizes a monospace font family into per-character
// coverage masks, for Gio views that compose text into images on the CPU
// instead of drawing Gio text paths.
package glyphs

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	gofont "gioui.org/font"
	gotext "github.com/go-text/typesetting/font"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/lichdotdev/lich/native/fonts"
)

// fontStyles are the styles of a family, indexed by styleIndex.
var fontStyles = [4]gofont.Font{
	{},
	{Weight: gofont.Bold},
	{Style: gofont.Italic},
	{Weight: gofont.Bold, Style: gofont.Italic},
}

func styleIndex(k Key) int {
	i := 0
	if k.Bold {
		i |= 1
	}
	if k.Italic {
		i |= 2
	}
	return i
}

// Key names one glyph: a character in one of the family's four styles.
type Key struct {
	R            rune
	Bold, Italic bool
}

// fontFace is one Rasterizer's view of a shared fontFile: its face at the
// rasterizer's size and its reader of color bitmaps, neither safe to share.
type fontFace struct {
	file *fontFile
	face font.Face
	// bitmaps reads file's color bitmaps; nil when it has none.
	bitmaps *gotext.Face
}

// Mask is a glyph's coverage, placed relative to its cell's top-left.
// Img is nil for a glyph that draws nothing. Color is set instead of Img for a
// glyph that carries its own colors, an emoji, and is drawn untinted.
type Mask struct {
	Img   *image.Alpha
	Color *image.RGBA
	Off   image.Point
}

// Rasterizer turns characters into coverage masks on the CPU, once each, for
// the renderers that keep glyphs as images instead of Gio text paths. It also
// owns the fallback Gio lacks: a character the family does not cover is drawn
// from whichever installed font fonts.Covering says covers it.
type Rasterizer struct {
	family string
	px     float64
	styles [4]*fontFace
	byFile map[string]*fontFace
	byRune map[rune]*fontFace
	masks  map[Key]*Mask
	buf    sfnt.Buffer
	cellW  int
	cellH  int
	ascent int

	// fallbacks are the faces fonts.Covering picked so far, in the order it
	// picked them, tried before asking it again: on Linux each ask is a
	// subprocess, and CJK text brings thousands of characters one face covers.
	fallbacks []*fontFace
}

// New loads the four styles fonts.Locate resolves for family. Call SetSize
// before Mask or Cell.
func New(family string) (*Rasterizer, error) {
	r := &Rasterizer{family: family, byFile: map[string]*fontFace{}}
	for i, style := range fontStyles {
		style.Typeface = gofont.Typeface(family)
		loc, err := fonts.Locate(style)
		if err != nil {
			return nil, err
		}
		if r.styles[i], err = r.load(loc.Path); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Rasterizer) load(path string) (*fontFace, error) {
	if f, ok := r.byFile[path]; ok {
		return f, nil
	}
	file, err := openFontFile(path)
	if err != nil {
		return nil, err
	}
	f := &fontFace{file: file}
	if file.color != nil {
		f.bitmaps = gotext.NewFace(file.color)
	}
	if r.px > 0 {
		if f.face, err = opentype.NewFace(file.sf, &opentype.FaceOptions{Size: r.px, DPI: 72}); err != nil {
			return nil, err
		}
	}
	r.byFile[path] = f
	return f, nil
}

// SetSize re-creates every face at px pixels per em and reports whether the
// size changed, which drops every cached mask.
func (r *Rasterizer) SetSize(px float64) (bool, error) {
	if px == r.px {
		return false, nil
	}
	r.px = px
	for _, f := range r.byFile {
		face, err := opentype.NewFace(f.file.sf, &opentype.FaceOptions{Size: px, DPI: 72})
		if err != nil {
			return false, err
		}
		f.face = face
	}
	m := r.styles[0].face.Metrics()
	adv, _ := r.styles[0].face.GlyphAdvance('M')
	r.cellW = adv.Ceil()
	r.ascent = m.Ascent.Ceil()
	r.cellH = (m.Ascent + m.Descent).Ceil()
	r.masks = map[Key]*Mask{}
	r.byRune = map[rune]*fontFace{}
	return true, nil
}

func (r *Rasterizer) covers(f *fontFace, c rune) bool {
	gi, err := f.file.sf.GlyphIndex(&r.buf, c)
	return err == nil && gi != 0
}

// faceFor picks the family's face for k, or a fallback that covers its
// character. It returns nil when nothing installed does.
func (r *Rasterizer) faceFor(k Key) (*fontFace, error) {
	primary := r.styles[styleIndex(k)]
	if r.covers(primary, k.R) {
		return primary, nil
	}
	if f, ok := r.byRune[k.R]; ok {
		return f, nil
	}
	for _, f := range r.fallbacks {
		if r.covers(f, k.R) {
			r.byRune[k.R] = f
			return f, nil
		}
	}
	path, err := fonts.Covering(k.R)
	if err != nil {
		return nil, err
	}
	var f *fontFace
	if path != "" {
		if f, err = r.load(path); err != nil {
			return nil, err
		}
		if r.covers(f, k.R) {
			r.fallbacks = append(r.fallbacks, f)
		} else {
			f = nil
		}
	}
	r.byRune[k.R] = f
	return f, nil
}

// Mask returns k's coverage, rasterizing it on first use.
func (r *Rasterizer) Mask(k Key) (*Mask, error) {
	if m, ok := r.masks[k]; ok {
		return m, nil
	}
	m := &Mask{}
	f, err := r.faceFor(k)
	if err != nil {
		return nil, err
	}
	// The color bitmap wins over the outline: Apple Color Emoji carries both,
	// and its outline is a monochrome stand-in.
	if f != nil {
		if m.Color, m.Off, err = r.colorBitmap(f, k.R); err != nil {
			return nil, err
		}
	}
	if f != nil && m.Color == nil {
		dr, src, sp, _, ok := f.face.Glyph(fixed.P(0, r.ascent), k.R)
		if ok && !dr.Empty() {
			a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
			draw.Draw(a, a.Bounds(), src, sp, draw.Src)
			m.Img, m.Off = a, dr.Min
		}
	}
	r.masks[k] = m
	return m, nil
}

// colorBitmap scales c's PNG bitmap to fit two cells, the width of an emoji,
// centered in them. It returns nil when f has no PNG bitmap for c. The grid
// carries no width per cell, so an emoji that is narrow (text presentation,
// no VS16) still overhangs into the next cell.
func (r *Rasterizer) colorBitmap(f *fontFace, c rune) (*image.RGBA, image.Point, error) {
	if f.bitmaps == nil {
		return nil, image.Point{}, nil
	}
	gid, ok := f.bitmaps.NominalGlyph(c)
	if !ok {
		return nil, image.Point{}, nil
	}
	bm, ok := f.bitmaps.GlyphDataBitmap(gid)
	if !ok || bm.Format != gotext.PNG {
		return nil, image.Point{}, nil
	}
	src, err := png.Decode(bytes.NewReader(bm.Data))
	if err != nil {
		return nil, image.Point{}, fmt.Errorf("decode bitmap of %U: %w", c, err)
	}
	boxW, boxH := 2*r.cellW, r.cellH
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	scale := min(float64(boxW)/float64(sw), float64(boxH)/float64(sh))
	w, h := max(int(float64(sw)*scale), 1), max(int(float64(sh)*scale), 1)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst, image.Pt((boxW-w)/2, (boxH-h)/2), nil
}

// CellW is the width of one monospace cell in pixels at the current size.
func (r *Rasterizer) CellW() int { return r.cellW }

// CellH is the height of one line in pixels at the current size.
func (r *Rasterizer) CellH() int { return r.cellH }
