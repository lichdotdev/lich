package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// GaugeBar is a rounded track in the border color, filled to percent in tone.
// The track is not Muted: the popover and, in the light theme, a full-size
// screen are drawn in that color, and the bar would be a fill with nothing to
// fill.
func (th *Theme) GaugeBar(gtx C, percent int, tone color.NRGBA) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(Space(1.5)))
	radius := Space(0.75)
	track := gtx
	track.Constraints.Min = size
	Fill(track, th.Over(th.Border), radius)
	fill := gtx
	fill.Constraints.Min = image.Pt(size.X*min(max(percent, 0), 100)/100, size.Y)
	if fill.Constraints.Min.X > 0 {
		Fill(fill, tone, radius)
	}
	return D{Size: size}
}

// Segment is one part of a SegmentBar: its share of the whole, in any unit.
type Segment struct {
	Share int
	Color color.NRGBA
}

// SegmentBar is GaugeBar split into parts, each as wide as its share of the
// sum. With no share at all it is the bare track.
func (th *Theme) SegmentBar(gtx C, parts ...Segment) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(Space(1.5)))
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(Space(0.75))).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	total := 0
	for _, p := range parts {
		total += p.Share
	}
	if total <= 0 {
		return D{Size: size}
	}
	x, sum := 0, 0
	for _, p := range parts {
		sum += p.Share
		// Each edge comes from the running sum, so rounding never leaves the
		// bar a pixel short of full.
		end := size.X * sum / total
		stackOp := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, p.Color, clip.Rect{Max: image.Pt(end-x, size.Y)}.Op())
		stackOp.Pop()
		x = end
	}
	return D{Size: size}
}

// DonutGauge draws on a 36-unit grid like the web's ring: a 20% track under
// an arc filled to percent clockwise from the top, with round ends.
const (
	donutViewBox      = 36
	donutRadius       = 15.9155
	donutStroke       = 4
	donutTrackOpacity = 0.2
)

// DonutGauge is a size square donut: a track under an arc filled to percent
// clockwise from the top, in fg.
func (th *Theme) DonutGauge(gtx C, size unit.Dp, percent int, fg color.NRGBA) D {
	px := gtx.Dp(size)
	scale := float32(px) / donutViewBox
	center := f32.Pt(float32(px)/2, float32(px)/2)
	radius, width := donutRadius*scale, donutStroke*scale
	StrokeArc(gtx, center, radius, width, 0, 2*math.Pi, th.Over(Alpha(fg, donutTrackOpacity)))
	share := float32(min(max(percent, 0), 100)) / 100
	if share > 0 {
		StrokeArc(gtx, center, radius, width, -math.Pi/2, share*2*math.Pi, fg)
		top := center.Sub(f32.Pt(0, radius))
		end := arcPoint(center, radius, share)
		DotAt(gtx, top, width/2, fg)
		DotAt(gtx, end, width/2, fg)
	}
	return D{Size: image.Pt(px, px)}
}

func arcPoint(center f32.Point, radius, share float32) f32.Point {
	angle := float64(share)*2*math.Pi - math.Pi/2
	return center.Add(f32.Pt(radius*float32(math.Cos(angle)), radius*float32(math.Sin(angle))))
}

// GaugeRowStyle lays out a gauge's four columns on one line: a label, the
// bar taking what is left, then a reading and a remainder set against the
// right edge, the three outer ones at fixed widths so rows stack in a grid.
type GaugeRowStyle struct {
	LabelW, ReadingW, LeftW unit.Dp
	Label, Bar              layout.Widget
	Reading, Left           layout.Widget
}

func (r GaugeRowStyle) Layout(gtx C) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		fixedColumn(r.LabelW, r.Label),
		Gap(3),
		layout.Flexed(1, r.Bar),
		Gap(3),
		fixedColumn(r.ReadingW, alignEnd(r.Reading)),
		Gap(3),
		fixedColumn(r.LeftW, alignEnd(r.Left)),
	)
}

func fixedColumn(width unit.Dp, w layout.Widget) layout.FlexChild {
	return layout.Rigid(func(gtx C) D {
		px := gtx.Dp(width)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = px, px
		return w(gtx)
	})
}

func alignEnd(w layout.Widget) layout.Widget {
	return func(gtx C) D { return layout.E.Layout(gtx, w) }
}
