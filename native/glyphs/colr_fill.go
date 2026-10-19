package glyphs

import (
	"fmt"
	"image"
	"math"
	"slices"

	"github.com/go-text/typesetting/font/opentype/tables"
)

// rgba is a premultiplied color, each channel in [0, 1].
type rgba struct{ r, g, b, a float64 }

func (c rgba) scaled(k float64) rgba { return rgba{c.r * k, c.g * k, c.b * k, c.a * k} }

func (c rgba) plus(d rgba) rgba { return rgba{c.r + d.r, c.g + d.g, c.b + d.b, c.a + d.a} }

// fill draws a solid or gradient paint, a leaf of the graph, over dst through
// clip. A gradient's geometry is in font units, mapped to pixels by m.
func (cp *colrPainter) fill(dst *image.RGBA, p tables.PaintTable, m affine, clip *image.Alpha) error {
	if solid, ok := static(p).(tables.PaintSolid); ok {
		fillSolid(dst, clip, cp.color(solid.PaletteIndex, f214(solid.Alpha)))
		return nil
	}
	at, line, ok := cp.gradient(static(p))
	if !ok {
		return fmt.Errorf("unknown COLR paint %T", p)
	}
	toFont, ok := m.invert()
	if !ok {
		return nil
	}
	for y := range cp.bounds.Dy() {
		for x := range cp.bounds.Dx() {
			cover := clip.AlphaAt(x, y).A
			if cover == 0 {
				continue
			}
			fx, fy := toFont.apply(float64(x)+0.5, float64(y)+0.5)
			t, painted := at(fx, fy)
			if painted {
				blend(dst, x, y, line.at(t).scaled(float64(cover)/0xff))
			}
		}
	}
	return nil
}

// gradient returns where on its color line a gradient paint puts each point of
// font space, false where it paints nothing.
func (cp *colrPainter) gradient(p tables.PaintTable) (func(x, y float64) (float64, bool), colorLine, bool) {
	switch p := p.(type) {
	case tables.PaintLinearGradient:
		return linearAt(p), cp.line(p.ColorLine), true
	case tables.PaintRadialGradient:
		return radialAt(p), cp.line(p.ColorLine), true
	case tables.PaintSweepGradient:
		return sweepAt(p), cp.line(p.ColorLine), true
	}
	return nil, colorLine{}, false
}

// linearAt projects onto p0→p3, the gradient vector p0→p1 turned to run
// perpendicular to p0→p2, as the COLR spec draws a linear gradient.
func linearAt(p tables.PaintLinearGradient) func(x, y float64) (float64, bool) {
	x0, y0 := float64(p.X0), float64(p.Y0)
	ax, ay := float64(p.X1)-x0, float64(p.Y1)-y0
	nx, ny := float64(p.Y2-p.Y0), -float64(p.X2-p.X0)
	if n := nx*nx + ny*ny; n != 0 {
		k := (ax*nx + ay*ny) / n
		ax, ay = k*nx, k*ny
	}
	length := ax*ax + ay*ay
	return func(x, y float64) (float64, bool) {
		if length == 0 {
			return 0, false
		}
		return ((x-x0)*ax + (y-y0)*ay) / length, true
	}
}

// radialAt is the two-circle gradient: the largest t whose circle, interpolated
// between the start and end circles, passes through the point with a radius
// that is not negative.
func radialAt(p tables.PaintRadialGradient) func(x, y float64) (float64, bool) {
	cx, cy, r0 := float64(p.X0), float64(p.Y0), float64(p.Radius0)
	dx, dy, dr := float64(p.X1)-cx, float64(p.Y1)-cy, float64(p.Radius1)-r0
	a := dx*dx + dy*dy - dr*dr
	return func(x, y float64) (float64, bool) {
		px, py := x-cx, y-cy
		b := px*dx + py*dy + r0*dr
		c := px*px + py*py - r0*r0
		if math.Abs(a) < 1e-9 {
			if b == 0 {
				return 0, false
			}
			t := c / (2 * b)
			return t, r0+t*dr >= 0
		}
		disc := b*b - a*c
		if disc < 0 {
			return 0, false
		}
		root := math.Sqrt(disc)
		lo, hi := (b-root)/a, (b+root)/a
		if lo > hi {
			lo, hi = hi, lo
		}
		for _, t := range [2]float64{hi, lo} {
			if r0+t*dr >= 0 {
				return t, true
			}
		}
		return 0, false
	}
}

// sweepAt is the counter-clockwise angle around the center, from the start
// angle to the end one, each stored as (degrees/180 - 1).
func sweepAt(p tables.PaintSweepGradient) func(x, y float64) (float64, bool) {
	cx, cy := float64(p.CenterX), float64(p.CenterY)
	start, end := (f214(p.StartAngle)+1)*180, (f214(p.EndAngle)+1)*180
	return func(x, y float64) (float64, bool) {
		if end == start {
			return 0, false
		}
		angle := math.Atan2(y-cy, x-cx) * 180 / math.Pi
		if angle < 0 {
			angle += 360
		}
		return (angle - start) / (end - start), true
	}
}

type colorStop struct {
	offset float64
	color  rgba
}

// colorLine is a gradient's stops, sorted, and what happens past either end.
type colorLine struct {
	stops  []colorStop
	extend tables.Extend
}

func (cp *colrPainter) line(l tables.ColorLine) colorLine {
	stops := make([]colorStop, len(l.ColorStops))
	for i, s := range l.ColorStops {
		stops[i] = colorStop{offset: f214(s.StopOffset), color: cp.color(s.PaletteIndex, f214(s.Alpha))}
	}
	slices.SortStableFunc(stops, func(a, b colorStop) int { return cmpFloat(a.offset, b.offset) })
	return colorLine{stops: stops, extend: l.Extend}
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// at is the color at t, with the line extended past its stops the way it says.
func (l colorLine) at(t float64) rgba {
	if len(l.stops) == 0 {
		return rgba{}
	}
	first, last := l.stops[0].offset, l.stops[len(l.stops)-1].offset
	if span := last - first; span > 0 && (t < first || t > last) {
		t = extendInto(t, first, span, l.extend)
	}
	if t <= first {
		return l.stops[0].color
	}
	for i := 1; i < len(l.stops); i++ {
		a, b := l.stops[i-1], l.stops[i]
		if t <= b.offset {
			if b.offset == a.offset {
				return b.color
			}
			k := (t - a.offset) / (b.offset - a.offset)
			return a.color.scaled(1 - k).plus(b.color.scaled(k))
		}
	}
	return l.stops[len(l.stops)-1].color
}

// extendInto brings t, outside [first, first+span], back onto the stops:
// repeated, mirrored, or left for the nearest stop to pad.
func extendInto(t, first, span float64, extend tables.Extend) float64 {
	u := (t - first) / span
	switch extend {
	case tables.ExtendRepeat:
		u -= math.Floor(u)
	case tables.ExtendReflect:
		u = math.Abs(u - 2*math.Floor(u/2))
		if u > 1 {
			u = 2 - u
		}
	default:
		return t
	}
	return first + u*span
}

func fillSolid(dst *image.RGBA, clip *image.Alpha, c rgba) {
	for i, cover := range clip.Pix {
		if cover != 0 {
			blendAt(dst, 4*i, c.scaled(float64(cover)/0xff))
		}
	}
}

func blend(dst *image.RGBA, x, y int, c rgba) { blendAt(dst, dst.PixOffset(x, y), c) }

// blendAt lays c over the pixel at offset i of dst.
func blendAt(dst *image.RGBA, i int, c rgba) {
	d := pixel(dst, i)
	setPixel(dst, i, c.plus(d.scaled(1-c.a)))
}

// over lays src over dst, both the same size.
func over(dst, src *image.RGBA) {
	for i := 0; i < len(src.Pix); i += 4 {
		blendAt(dst, i, pixel(src, i))
	}
}

func pixel(img *image.RGBA, i int) rgba {
	p := img.Pix[i : i+4 : i+4]
	return rgba{float64(p[0]) / 0xff, float64(p[1]) / 0xff, float64(p[2]) / 0xff, float64(p[3]) / 0xff}
}

func setPixel(img *image.RGBA, i int, c rgba) {
	p := img.Pix[i : i+4 : i+4]
	for j, v := range [4]float64{c.r, c.g, c.b, c.a} {
		p[j] = uint8(math.Round(math.Max(0, math.Min(1, v)) * 0xff))
	}
}

// compositeLayers merges source onto backdrop, in place, by mode: the
// Porter-Duff operators, and the separable blend modes laid source-over.
func compositeLayers(source, backdrop *image.RGBA, mode tables.CompositeMode) {
	for i := 0; i < len(source.Pix); i += 4 {
		setPixel(backdrop, i, compositePixel(pixel(source, i), pixel(backdrop, i), mode))
	}
}

func compositePixel(s, d rgba, mode tables.CompositeMode) rgba {
	if fa, fb, ok := porterDuff(mode, s.a, d.a); ok {
		return s.scaled(fa).plus(d.scaled(fb))
	}
	mix := blendChannel(mode)
	// The W3C compositing formula for a separable blend mode, premultiplied.
	ch := func(sc, dc float64) float64 {
		var b float64
		if s.a > 0 && d.a > 0 {
			b = mix(sc/s.a, dc/d.a)
		}
		return sc*(1-d.a) + dc*(1-s.a) + s.a*d.a*b
	}
	return rgba{ch(s.r, d.r), ch(s.g, d.g), ch(s.b, d.b), s.a + d.a - s.a*d.a}
}

// porterDuff is the factors source and backdrop are weighted by in mode,
// false for a blend mode.
func porterDuff(mode tables.CompositeMode, sa, da float64) (float64, float64, bool) {
	switch mode {
	case tables.CompositeClear:
		return 0, 0, true
	case tables.CompositeSrc:
		return 1, 0, true
	case tables.CompositeDest:
		return 0, 1, true
	case tables.CompositeSrcOver:
		return 1, 1 - sa, true
	case tables.CompositeDestOver:
		return 1 - da, 1, true
	case tables.CompositeSrcIn:
		return da, 0, true
	case tables.CompositeDestIn:
		return 0, sa, true
	case tables.CompositeSrcOut:
		return 1 - da, 0, true
	case tables.CompositeDestOut:
		return 0, 1 - sa, true
	case tables.CompositeSrcAtop:
		return da, 1 - sa, true
	case tables.CompositeDestAtop:
		return 1 - da, sa, true
	case tables.CompositeXor:
		return 1 - da, 1 - sa, true
	case tables.CompositePlus:
		return 1, 1, true
	}
	return 0, 0, false
}

// blendChannel is mode's mix of one unpremultiplied channel.
// ponytail: the non-separable modes (hue, saturation, color, luminosity) mix
// as normal, the source showing as is; give them their HSL math once an emoji
// font is seen using one.
func blendChannel(mode tables.CompositeMode) func(s, d float64) float64 {
	switch mode {
	case tables.CompositeScreen:
		return func(s, d float64) float64 { return s + d - s*d }
	case tables.CompositeMultiply:
		return func(s, d float64) float64 { return s * d }
	case tables.CompositeDarken:
		return math.Min
	case tables.CompositeLighten:
		return math.Max
	case tables.CompositeDifference:
		return func(s, d float64) float64 { return math.Abs(s - d) }
	case tables.CompositeExclusion:
		return func(s, d float64) float64 { return s + d - 2*s*d }
	case tables.CompositeOverlay:
		return func(s, d float64) float64 { return hardLight(d, s) }
	case tables.CompositeHardLight:
		return hardLight
	case tables.CompositeSoftLight:
		return softLight
	case tables.CompositeColorDodge:
		return colorDodge
	case tables.CompositeColorBurn:
		return colorBurn
	}
	return func(s, _ float64) float64 { return s }
}

func hardLight(s, d float64) float64 {
	if s <= 0.5 {
		return 2 * s * d
	}
	return 1 - 2*(1-s)*(1-d)
}

func softLight(s, d float64) float64 {
	if s <= 0.5 {
		return d - (1-2*s)*d*(1-d)
	}
	g := math.Sqrt(d)
	if d <= 0.25 {
		g = ((16*d-12)*d + 4) * d
	}
	return d + (2*s-1)*(g-d)
}

func colorDodge(s, d float64) float64 {
	switch {
	case d == 0:
		return 0
	case s >= 1:
		return 1
	}
	return math.Min(1, d/(1-s))
}

func colorBurn(s, d float64) float64 {
	switch {
	case d >= 1:
		return 1
	case s <= 0:
		return 0
	}
	return 1 - math.Min(1, (1-d)/s)
}
