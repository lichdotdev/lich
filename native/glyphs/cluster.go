package glyphs

import (
	"image"

	"github.com/go-text/typesetting/di"
	gotext "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// cluster draws a grapheme of several codepoints shaped as one run in the
// face that covers its first: a ZWJ sequence or a flag becomes the font's
// ligature glyph, a combining mark lands where the font's mark positioning
// puts it.
func (r *Rasterizer) cluster(k Key) (*Mask, error) {
	m := &Mask{}
	f, err := r.faceFor(k)
	if err != nil || f == nil {
		return m, err
	}
	face, err := r.textFace(f)
	if err != nil {
		return nil, err
	}
	text := []rune(string(k.R) + k.Rest)
	out := r.shaper.Shape(shaping.Input{
		Text:      text,
		RunEnd:    len(text),
		Direction: di.DirectionLTR,
		Face:      face,
		Size:      fixed.Int26_6(r.px * 64),
		Script:    language.LookupScript(k.R),
	})
	if len(out.Glyphs) == 0 {
		return m, nil
	}
	// shortcut: a color grapheme the font has no single glyph for draws its
	// first glyph only, the cell being two columns wide whatever it holds.
	if f.bitmaps != nil {
		if m.Color, m.Off, err = r.colorGlyphID(f, out.Glyphs[0].GlyphID); err != nil || m.Color != nil {
			return m, err
		}
	}
	m.Img = r.outlines(face, out.Glyphs)
	return m, nil
}

func (r *Rasterizer) textFace(f *fontFace) (*gotext.Face, error) {
	if f.text != nil {
		return f.text, nil
	}
	font, err := f.file.textFont()
	if err != nil {
		return nil, err
	}
	f.text = gotext.NewFace(font)
	return f.text, nil
}

// outlines fills the shaped glyphs into a two-cell mask, its origin the cell's
// top-left. A glyph the face lacks (.notdef) is left out, so a mark the font
// has no glyph for leaves its base drawn alone rather than beside a box.
func (r *Rasterizer) outlines(face *gotext.Face, gs []shaping.Glyph) *image.Alpha {
	bounds := image.Rect(0, 0, 2*r.cellW, r.cellH)
	p := colrPainter{face: face, bounds: bounds}
	scale := r.px / float64(face.Upem())
	dst := image.NewAlpha(bounds)
	pen := fixed.Int26_6(0)
	for _, g := range gs {
		if g.GlyphID != 0 {
			x := float64(pen+g.XOffset) / 64
			y := float64(r.ascent) - float64(g.YOffset)/64
			cover := p.cover(tables.GlyphID(g.GlyphID), affine{xx: scale, yy: -scale, dx: x, dy: y})
			for i, a := range cover.Pix {
				dst.Pix[i] = max(dst.Pix[i], a)
			}
		}
		pen += g.Advance
	}
	return dst
}
