package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// InsetBorder strokes a 1px line just inside size, a box-shadow inset 0 0 0
// 1px.
func InsetBorder(gtx C, size image.Point, r int, c color.NRGBA) {
	InsetRing(gtx, size, r, gtx.Dp(1), c)
}

// InsetRing strokes a line width pixels wide just inside size, ring-inset:
// a stroke twice as wide on the edge, clipped to the inside half.
func InsetRing(gtx C, size image.Point, r, width int, c color.NRGBA) {
	rr := clip.UniformRRect(image.Rectangle{Max: size}, r)
	defer rr.Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(2 * width)}.Op())
}

// EdgeGap is a run of an island's top edge, From to To in its x, that IslandBorder
// leaves open: the room a tab grown out of the island takes.
type EdgeGap struct{ From, To int }

// IslandBorder strokes a 1px line just inside size, nw the radius of the
// top-left corner and r of the others. A gap opens the top edge over it, where
// TabBorder and FilletBorder carry the line up the tab.
func IslandBorder(gtx C, size image.Point, nw, r int, gap *EdgeGap, c color.NRGBA) {
	w := float32(gtx.Dp(1))
	h := w / 2
	x0, y0, x1, y1 := h, h, float32(size.X)-h, float32(size.Y)-h
	fr, fnw := float32(r), float32(nw)
	q := float32(bezierQuarter)
	start := x0 + fnw
	if gap != nil {
		start = float32(gap.To)
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(start, y0))
	p.LineTo(f32.Pt(x1-fr, y0))
	p.CubeTo(f32.Pt(x1-fr+q*fr, y0), f32.Pt(x1, y0+fr-q*fr), f32.Pt(x1, y0+fr))
	p.LineTo(f32.Pt(x1, y1-fr))
	p.CubeTo(f32.Pt(x1, y1-fr+q*fr), f32.Pt(x1-fr+q*fr, y1), f32.Pt(x1-fr, y1))
	p.LineTo(f32.Pt(x0+fr, y1))
	p.CubeTo(f32.Pt(x0+fr-q*fr, y1), f32.Pt(x0, y1-fr+q*fr), f32.Pt(x0, y1-fr))
	p.LineTo(f32.Pt(x0, y0+fnw))
	p.CubeTo(f32.Pt(x0, y0+fnw-q*fnw), f32.Pt(x0+fnw-q*fnw, y0), f32.Pt(x0+fnw, y0))
	if gap == nil {
		p.Close()
	} else if float32(gap.From) > x0+fnw {
		p.LineTo(f32.Pt(float32(gap.From), y0))
	}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: w}.Op())
}

// TabBorder strokes the left, top and right of a tab that has grown into its
// island, 1px just inside size. Each side stops r short of the island's top
// edge, where FilletBorder takes the line on, and runs the whole way down on
// the left when flush is true: a tab flush with its island has no left fillet.
func TabBorder(gtx C, size image.Point, r int, flush bool, c color.NRGBA) {
	w := float32(gtx.Dp(1))
	h := w / 2
	x1, bottom := float32(size.X)-h, float32(size.Y)
	fr, q := float32(r), float32(bezierQuarter)
	foot := bottom - fr
	var p clip.Path
	p.Begin(gtx.Ops)
	if flush {
		p.MoveTo(f32.Pt(h, bottom))
	} else {
		p.MoveTo(f32.Pt(h, foot))
	}
	p.LineTo(f32.Pt(h, fr))
	p.CubeTo(f32.Pt(h, fr-q*fr), f32.Pt(fr-q*fr, h), f32.Pt(fr, h))
	p.LineTo(f32.Pt(x1-fr, h))
	p.CubeTo(f32.Pt(x1-fr+q*fr, h), f32.Pt(x1, fr-q*fr), f32.Pt(x1, fr))
	p.LineTo(f32.Pt(x1, foot))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: w}.Op())
}

// FilletBorder strokes the curve Fillet fills at p, the line turning a tab's
// side into the island's top edge: it ends on the row below the fillet's box,
// where IslandBorder's top edge runs, so the caller's clip reaches one line
// past the box.
func FilletBorder(gtx C, p image.Point, r int, right bool, c color.NRGBA) {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	w := float32(gtx.Dp(1))
	h := w / 2
	s, q := float32(r), float32(bezierQuarter)
	var path clip.Path
	path.Begin(gtx.Ops)
	if right {
		path.MoveTo(f32.Pt(-h, 0))
		path.CubeTo(f32.Pt(-h, s*q), f32.Pt(s-s*q, s+h), f32.Pt(s, s+h))
	} else {
		path.MoveTo(f32.Pt(s+h, 0))
		path.CubeTo(f32.Pt(s+h, s*q), f32.Pt(s*q, s+h), f32.Pt(0, s+h))
	}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: path.End(), Width: w}.Op())
}
