package glyphs

import (
	"errors"
	"fmt"
	"image"
	"math"

	gotext "github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"golang.org/x/image/vector"
)

// maxPaintDepth bounds a COLR paint graph, which the format lets a broken font
// make cyclic (a PaintColrGlyph reaching itself).
const maxPaintDepth = 64

// foregroundIndex is the palette index COLR reserves for the text's own color.
const foregroundIndex = 0xFFFF

var errPaintTooDeep = errors.New("COLR paint graph deeper than allowed, a cycle")

// colorLayers draws c's COLR color glyph, the format Windows' Segoe UI Emoji
// uses (version 0 layers, or a version 1 paint graph), into the two cells
// colorBitmap scales a PNG into, centered in them. It returns nil when f has no
// color glyph for c.
func (r *Rasterizer) colorLayers(f *fontFace, c rune) (*image.RGBA, image.Point, error) {
	if f.bitmaps == nil || f.bitmaps.COLR == nil {
		return nil, image.Point{}, nil
	}
	gid, ok := f.bitmaps.NominalGlyph(c)
	if !ok {
		return nil, image.Point{}, nil
	}
	root, ok := f.bitmaps.COLR.Search(tables.GlyphID(gid))
	if !ok {
		return nil, image.Point{}, nil
	}
	ext, _ := f.bitmaps.FontHExtents()
	emW, emH := float64(f.bitmaps.HorizontalAdvance(gid)), float64(ext.Ascender-ext.Descender)
	if emW <= 0 || emH <= 0 {
		return nil, image.Point{}, nil
	}
	boxW, boxH := 2*r.cellW, r.cellH
	scale := min(float64(boxW)/emW, float64(boxH)/emH)
	w, h := max(int(math.Ceil(emW*scale)), 1), max(int(math.Ceil(emH*scale)), 1)
	p := colrPainter{face: f.bitmaps, bounds: image.Rect(0, 0, w, h)}
	if len(f.bitmaps.CPAL) > 0 {
		p.palette = f.bitmaps.CPAL[0]
	}
	// Font units to pixels: y grows down, and the ascender is the top row.
	fontToPixels := affine{xx: scale, yy: -scale, dy: float64(ext.Ascender) * scale}
	dst := image.NewRGBA(p.bounds)
	if err := p.paint(dst, root, fontToPixels, p.opaque(), 0); err != nil {
		return nil, image.Point{}, fmt.Errorf("draw COLR glyph of %U: %w", c, err)
	}
	return dst, image.Pt((boxW-w)/2, (boxH-h)/2), nil
}

// colrPainter walks one COLR glyph's paint graph onto RGBA layers.
type colrPainter struct {
	face    *gotext.Face
	palette []tables.ColorRecord
	bounds  image.Rectangle
}

// paint draws p, placed by m (font units to pixels) and masked by clip, over
// dst.
func (cp *colrPainter) paint(dst *image.RGBA, p tables.PaintTable, m affine, clip *image.Alpha, depth int) error {
	if depth > maxPaintDepth {
		return errPaintTooDeep
	}
	if t, child, ok := transformOf(p); ok {
		return cp.paint(dst, child, m.then(t), clip, depth+1)
	}
	switch p := p.(type) {
	case tables.PaintColrLayersResolved:
		for _, layer := range p {
			fillSolid(dst, intersect(clip, cp.cover(layer.GlyphID, m)), cp.color(layer.PaletteIndex, 1))
		}
	case tables.PaintColrLayers:
		layers, err := cp.face.COLR.LayerList.Resolve(p)
		if err != nil {
			return err
		}
		for _, layer := range layers {
			if err := cp.paint(dst, layer, m, clip, depth+1); err != nil {
				return err
			}
		}
	case tables.PaintGlyph:
		return cp.paint(dst, p.Paint, m, intersect(clip, cp.cover(tables.GlyphID(p.GlyphID), m)), depth+1)
	case tables.PaintColrGlyph:
		if child, ok := cp.face.COLR.Search(tables.GlyphID(p.GlyphID)); ok {
			return cp.paint(dst, child, m, clip, depth+1)
		}
	case tables.PaintComposite:
		return cp.composite(dst, p, m, clip, depth)
	default:
		return cp.fill(dst, p, m, clip)
	}
	return nil
}

// composite draws p's backdrop and source on layers of their own, merges them
// by p's mode and lays the result over dst.
func (cp *colrPainter) composite(dst *image.RGBA, p tables.PaintComposite, m affine, clip *image.Alpha, depth int) error {
	backdrop, source := image.NewRGBA(cp.bounds), image.NewRGBA(cp.bounds)
	if err := cp.paint(backdrop, p.BackdropPaint, m, clip, depth+1); err != nil {
		return err
	}
	if err := cp.paint(source, p.SourcePaint, m, clip, depth+1); err != nil {
		return err
	}
	compositeLayers(source, backdrop, p.CompositeMode)
	over(dst, backdrop)
	return nil
}

// cover is the coverage of glyph gid's outline placed by m.
func (cp *colrPainter) cover(gid tables.GlyphID, m affine) *image.Alpha {
	mask := image.NewAlpha(cp.bounds)
	outline, ok := cp.face.GlyphDataOutline(gotext.GID(gid))
	if !ok || len(outline.Segments) == 0 {
		return mask
	}
	z := vector.NewRasterizer(cp.bounds.Dx(), cp.bounds.Dy())
	at := func(p ot.SegmentPoint) (float32, float32) {
		x, y := m.apply(float64(p.X), float64(p.Y))
		return float32(x), float32(y)
	}
	started := false
	for _, s := range outline.Segments {
		switch s.Op {
		case ot.SegmentOpMoveTo:
			if started {
				z.ClosePath()
			}
			started = true
			z.MoveTo(at(s.Args[0]))
		case ot.SegmentOpLineTo:
			z.LineTo(at(s.Args[0]))
		case ot.SegmentOpQuadTo:
			bx, by := at(s.Args[0])
			cx, cy := at(s.Args[1])
			z.QuadTo(bx, by, cx, cy)
		case ot.SegmentOpCubeTo:
			bx, by := at(s.Args[0])
			cx, cy := at(s.Args[1])
			dx, dy := at(s.Args[2])
			z.CubeTo(bx, by, cx, cy, dx, dy)
		}
	}
	z.ClosePath()
	z.Draw(mask, cp.bounds, image.Opaque, image.Point{})
	return mask
}

func (cp *colrPainter) opaque() *image.Alpha {
	mask := image.NewAlpha(cp.bounds)
	for i := range mask.Pix {
		mask.Pix[i] = 0xff
	}
	return mask
}

// color is palette entry index at alpha, premultiplied, in [0, 1]. The text's
// own color is not known here, the mask being drawn untinted, so foregroundIndex
// draws black.
func (cp *colrPainter) color(index uint16, alpha float64) rgba {
	if index == foregroundIndex || int(index) >= len(cp.palette) {
		return rgba{a: alpha}
	}
	c := cp.palette[index]
	a := float64(c.Alpha) / 0xff * alpha
	return rgba{r: float64(c.Red) / 0xff * a, g: float64(c.Green) / 0xff * a, b: float64(c.Blue) / 0xff * a, a: a}
}

// intersect is the coverage both masks give each pixel.
func intersect(a, b *image.Alpha) *image.Alpha {
	out := image.NewAlpha(a.Rect)
	for i := range out.Pix {
		out.Pix[i] = uint8(uint16(a.Pix[i]) * uint16(b.Pix[i]) / 0xff)
	}
	return out
}

func f214(c tables.Coord) float64 { return float64(c) / (1 << 14) }

// affine maps (x, y) to (xx·x + xy·y + dx, yx·x + yy·y + dy).
type affine struct{ xx, yx, xy, yy, dx, dy float64 }

func (m affine) apply(x, y float64) (float64, float64) {
	return m.xx*x + m.xy*y + m.dx, m.yx*x + m.yy*y + m.dy
}

// then is m applied after t: what a paint transformed by t inside m draws with.
func (m affine) then(t affine) affine {
	return affine{
		xx: m.xx*t.xx + m.xy*t.yx, yx: m.yx*t.xx + m.yy*t.yx,
		xy: m.xx*t.xy + m.xy*t.yy, yy: m.yx*t.xy + m.yy*t.yy,
		dx: m.xx*t.dx + m.xy*t.dy + m.dx, dy: m.yx*t.dx + m.yy*t.dy + m.dy,
	}
}

func (m affine) invert() (affine, bool) {
	det := m.xx*m.yy - m.xy*m.yx
	if math.Abs(det) < 1e-12 {
		return affine{}, false
	}
	inv := affine{xx: m.yy / det, yx: -m.yx / det, xy: -m.xy / det, yy: m.xx / det}
	inv.dx, inv.dy = -(inv.xx*m.dx + inv.xy*m.dy), -(inv.yx*m.dx + inv.yy*m.dy)
	return inv, true
}

func translate(dx, dy float64) affine { return affine{xx: 1, yy: 1, dx: dx, dy: dy} }

func scaleBy(sx, sy float64) affine { return affine{xx: sx, yy: sy} }

// rotate turns counter-clockwise by turns·180°, COLR's unit.
func rotate(turns float64) affine {
	sin, cos := math.Sincos(turns * math.Pi)
	return affine{xx: cos, yx: sin, xy: -sin, yy: cos}
}

func skew(xTurns, yTurns float64) affine {
	return affine{xx: 1, yy: 1, xy: -math.Tan(xTurns * math.Pi), yx: math.Tan(yTurns * math.Pi)}
}

// around is t applied about (cx, cy) instead of the origin.
func around(t affine, cx, cy int16) affine {
	return translate(float64(cx), float64(cy)).then(t).then(translate(-float64(cx), -float64(cy)))
}

// transformOf reads p as a transform of a child paint.
func transformOf(p tables.PaintTable) (affine, tables.PaintTable, bool) {
	switch p := static(p).(type) {
	case tables.PaintTransform:
		t := p.Transform
		return affine{xx: float64(t.Xx), yx: float64(t.Yx), xy: float64(t.Xy), yy: float64(t.Yy), dx: float64(t.Dx), dy: float64(t.Dy)}, p.Paint, true
	case tables.PaintTranslate:
		return translate(float64(p.Dx), float64(p.Dy)), p.Paint, true
	case tables.PaintScale:
		return scaleBy(f214(p.ScaleX), f214(p.ScaleY)), p.Paint, true
	case tables.PaintScaleAroundCenter:
		return around(scaleBy(f214(p.ScaleX), f214(p.ScaleY)), p.CenterX, p.CenterY), p.Paint, true
	case tables.PaintScaleUniform:
		return scaleBy(f214(p.Scale), f214(p.Scale)), p.Paint, true
	case tables.PaintScaleUniformAroundCenter:
		return around(scaleBy(f214(p.Scale), f214(p.Scale)), p.CenterX, p.CenterY), p.Paint, true
	case tables.PaintRotate:
		return rotate(f214(p.Angle)), p.Paint, true
	case tables.PaintRotateAroundCenter:
		return around(rotate(f214(p.Angle)), p.CenterX, p.CenterY), p.Paint, true
	case tables.PaintSkew:
		return skew(f214(p.XSkewAngle), f214(p.YSkewAngle)), p.Paint, true
	case tables.PaintSkewAroundCenter:
		return around(skew(f214(p.XSkewAngle), f214(p.YSkewAngle)), p.CenterX, p.CenterY), p.Paint, true
	}
	return affine{}, nil, false
}

// static reads a variable paint as its fixed twin at the font's default
// instance, lich setting no variation axis; any other paint comes back as is.
func static(p tables.PaintTable) tables.PaintTable {
	switch p := p.(type) {
	case tables.PaintVarSolid:
		return tables.PaintSolid{PaletteIndex: p.PaletteIndex, Alpha: p.Alpha}
	case tables.PaintVarLinearGradient:
		return tables.PaintLinearGradient{ColorLine: fixedLine(p.ColorLine), X0: p.X0, Y0: p.Y0, X1: p.X1, Y1: p.Y1, X2: p.X2, Y2: p.Y2}
	case tables.PaintVarRadialGradient:
		return tables.PaintRadialGradient{ColorLine: fixedLine(p.ColorLine), X0: p.X0, Y0: p.Y0, Radius0: p.Radius0, X1: p.X1, Y1: p.Y1, Radius1: p.Radius1}
	case tables.PaintVarSweepGradient:
		return tables.PaintSweepGradient{ColorLine: fixedLine(p.ColorLine), CenterX: p.CenterX, CenterY: p.CenterY, StartAngle: p.StartAngle, EndAngle: p.EndAngle}
	case tables.PaintVarTransform:
		t := p.Transform
		return tables.PaintTransform{Paint: p.Paint, Transform: tables.Affine2x3{Xx: t.Xx, Yx: t.Yx, Xy: t.Xy, Yy: t.Yy, Dx: t.Dx, Dy: t.Dy}}
	case tables.PaintVarTranslate:
		return tables.PaintTranslate{Paint: p.Paint, Dx: p.Dx, Dy: p.Dy}
	case tables.PaintVarScale:
		return tables.PaintScale{Paint: p.Paint, ScaleX: p.ScaleX, ScaleY: p.ScaleY}
	case tables.PaintVarScaleAroundCenter:
		return tables.PaintScaleAroundCenter{Paint: p.Paint, ScaleX: p.ScaleX, ScaleY: p.ScaleY, CenterX: p.CenterX, CenterY: p.CenterY}
	case tables.PaintVarScaleUniform:
		return tables.PaintScaleUniform{Paint: p.Paint, Scale: p.Scale}
	case tables.PaintVarScaleUniformAroundCenter:
		return tables.PaintScaleUniformAroundCenter{Paint: p.Paint, Scale: p.Scale, CenterX: p.CenterX, CenterY: p.CenterY}
	case tables.PaintVarRotate:
		return tables.PaintRotate{Paint: p.Paint, Angle: p.Angle}
	case tables.PaintVarRotateAroundCenter:
		return tables.PaintRotateAroundCenter{Paint: p.Paint, Angle: p.Angle, CenterX: p.CenterX, CenterY: p.CenterY}
	case tables.PaintVarSkew:
		return tables.PaintSkew{Paint: p.Paint, XSkewAngle: p.XSkewAngle, YSkewAngle: p.YSkewAngle}
	case tables.PaintVarSkewAroundCenter:
		return tables.PaintSkewAroundCenter{Paint: p.Paint, XSkewAngle: p.XSkewAngle, YSkewAngle: p.YSkewAngle, CenterX: p.CenterX, CenterY: p.CenterY}
	}
	return p
}

func fixedLine(l tables.VarColorLine) tables.ColorLine {
	stops := make([]tables.ColorStop, len(l.ColorStops))
	for i, s := range l.ColorStops {
		stops[i] = tables.ColorStop{StopOffset: s.StopOffset, PaletteIndex: s.PaletteIndex, Alpha: s.Alpha}
	}
	return tables.ColorLine{Extend: l.Extend, ColorStops: stops}
}
