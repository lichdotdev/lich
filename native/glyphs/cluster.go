package glyphs

import (
	"image"
	"log"
	"math"
	"strings"
	"unicode"

	"github.com/go-text/typesetting/di"
	gotext "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"github.com/lichdotdev/lich/native/fonts"
)

const (
	// maxClusterMasks bounds the masks kept for graphemes of several
	// codepoints, whose combinations are the program's to choose.
	maxClusterMasks = 1024
	// maxClusterGlyphs bounds the glyphs of one grapheme filled into its mask.
	maxClusterGlyphs = 32

	emojiSelector   = '\ufe0f'
	zeroWidthJoiner = '\u200d'
	regionalFirst   = '\U0001F1E6'
	regionalLast    = '\U0001F1FF'
	skinToneFirst   = '\U0001F3FB'
	skinToneLast    = '\U0001F3FF'
	flagTagFirst    = '\U000E0020'
	flagTagLast     = '\U000E007F'
)

// Emoji reports whether the grapheme is an emoji sequence: its glyph is in the
// color font even when a text font covers its first codepoint, as most fonts
// do a digit, a heart or a warning sign.
func (k Key) Emoji() bool {
	if k.Rest == "" {
		return false
	}
	// A joiner alone proves nothing: Indic scripts use it inside a conjunct.
	joinsEmoji := unicode.Is(unicode.So, k.R)
	return k.R >= regionalFirst && k.R <= regionalLast || strings.ContainsFunc(k.Rest, func(c rune) bool {
		return c == emojiSelector || c == zeroWidthJoiner && joinsEmoji ||
			c >= skinToneFirst && c <= skinToneLast || c >= flagTagFirst && c <= flagTagLast
	})
}

// cluster draws a grapheme of several codepoints shaped as one run: an emoji
// sequence becomes the color font's ligature glyph, a combining mark lands
// where the font's mark positioning puts it. What cannot be shaped draws its
// first codepoint, or nothing: one accent in a program's output must not fail
// the frame.
func (r *Rasterizer) cluster(k Key) *Mask {
	if k.Emoji() {
		if f := r.emojiFace(k.R); f != nil {
			if m := r.shaped(f, k); m != nil {
				return m
			}
		}
	}
	f, err := r.faceFor(k)
	if err != nil {
		r.warnOnce("find a font", err)
	} else if f != nil {
		if m := r.shaped(f, k); m != nil {
			return m
		}
	}
	m, err := r.Mask(Key{R: k.R, Bold: k.Bold, Italic: k.Italic})
	if err != nil {
		r.warnOnce("find a font", err)
		return &Mask{}
	}
	return m
}

// warnOnce logs err the first time what fails: a grapheme is drawn every dirty
// frame, and its failure would repeat once per cell.
func (r *Rasterizer) warnOnce(what string, err error) {
	if _, ok := r.warned[what]; ok {
		return
	}
	if r.warned == nil {
		r.warned = map[string]struct{}{}
	}
	r.warned[what] = struct{}{}
	log.Printf("glyphs: %s: %v", what, err)
}

// emojiFace is the color font when it covers c. Asked once; a machine without
// one draws from the text fonts.
func (r *Rasterizer) emojiFace(c rune) *fontFace {
	if !r.emojiFound {
		r.emojiFound = true
		r.emoji = r.loadEmojiFace()
	}
	if r.emoji == nil || !r.covers(r.emoji, c) {
		return nil
	}
	return r.emoji
}

func (r *Rasterizer) loadEmojiFace() *fontFace {
	path, err := fonts.ColorEmoji()
	if err != nil {
		r.warnOnce("find the emoji font", err)
		return nil
	}
	if path == "" {
		return nil
	}
	f, err := r.load(path)
	if err != nil {
		r.warnOnce("load the emoji font", err)
		return nil
	}
	if f.bitmaps == nil {
		return nil
	}
	return f
}

// shaped is k drawn from f, nil when f cannot draw it.
//
// shortcut: a color grapheme the font has no single glyph for draws its first
// glyph only, the cell being two columns wide whatever it holds.
func (r *Rasterizer) shaped(f *fontFace, k Key) *Mask {
	face, err := r.textFace(f)
	if err != nil {
		r.warnOnce("shape with "+f.file.path, err)
		return nil
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
		return nil
	}
	if f.bitmaps != nil {
		img, off, err := r.colorGlyphID(f, out.Glyphs[0].GlyphID)
		if err != nil {
			r.warnOnce("draw a color glyph of "+f.file.path, err)
		}
		if img != nil {
			return &Mask{Color: img, Off: off}
		}
	}
	img, off := r.outlines(face, out.Glyphs[:min(len(out.Glyphs), maxClusterGlyphs)])
	if img == nil {
		return nil
	}
	return &Mask{Img: img, Off: off}
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

// placedGlyph is a shaped glyph's origin in pixels from the cell's top-left.
type placedGlyph struct {
	gid  gotext.GID
	x, y float64
}

// outlines fills the shaped glyphs into a mask of their own bounds and returns
// where it sits from the cell's top-left, as a single codepoint's mask does.
// nil when they draw nothing. A glyph the face lacks (.notdef) is left out, so
// a mark the font has no glyph for leaves its base drawn alone rather than
// beside a box.
func (r *Rasterizer) outlines(face *gotext.Face, gs []shaping.Glyph) (*image.Alpha, image.Point) {
	scale := r.px / float64(face.Upem())
	placed := make([]placedGlyph, 0, len(gs))
	var box image.Rectangle
	pen := fixed.Int26_6(0)
	for _, g := range gs {
		x := float64(pen+g.XOffset) / 64
		y := float64(r.ascent) - float64(g.YOffset)/64
		ext, ok := face.GlyphExtents(g.GlyphID)
		// A font positions no mark on a blank, so past the blank's advance
		// the mark would land in the next cell instead of on this one.
		if ok && ext.Width != 0 {
			pen += g.Advance
		}
		if g.GlyphID == 0 || !ok {
			continue
		}
		placed = append(placed, placedGlyph{g.GlyphID, x, y})
		box = box.Union(image.Rect(
			floor(x+float64(ext.XBearing)*scale), floor(y-float64(ext.YBearing)*scale),
			ceil(x+float64(ext.XBearing+ext.Width)*scale), ceil(y-float64(ext.YBearing+ext.Height)*scale),
		))
	}
	// A stack of marks may reach any height: a cell to each side and a row
	// above and below bound what the mask costs. The rows renderer still cuts
	// it at its own row, where the atlas paints over the rows beside it.
	box = box.Intersect(image.Rect(-r.cellW, -r.cellH, 3*r.cellW, 2*r.cellH))
	if box.Empty() {
		return nil, image.Point{}
	}
	bounds := image.Rect(0, 0, box.Dx(), box.Dy())
	p := colrPainter{face: face, bounds: bounds}
	dst := image.NewAlpha(bounds)
	for _, g := range placed {
		cover := p.cover(tables.GlyphID(g.gid), affine{xx: scale, yy: -scale, dx: g.x - float64(box.Min.X), dy: g.y - float64(box.Min.Y)})
		for i, a := range cover.Pix {
			dst.Pix[i] = max(dst.Pix[i], a)
		}
	}
	return dst, box.Min
}

func floor(v float64) int { return int(math.Floor(v)) }

func ceil(v float64) int { return int(math.Ceil(v)) }
