package ui

import (
	"image"
	"image/color"
	"math"
	"time"

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
		FixedColumn(r.LabelW, r.Label),
		Gap(3),
		layout.Flexed(1, r.Bar),
		Gap(3),
		FixedColumn(r.ReadingW, AlignEnd(r.Reading)),
		Gap(3),
		FixedColumn(r.LeftW, AlignEnd(r.Left)),
	)
}

// segmentedGap is the room between a SegmentedBar's segments.
var segmentedGap = Space(0.5)

// SegmentsLit is how many of count segments a percent fills: rounded to the
// nearest, and at least one for any share at all.
func SegmentsLit(percent, count int) int {
	percent = min(max(percent, 0), 100)
	lit := (percent*count + 50) / 100
	if percent > 0 && lit == 0 {
		return 1
	}
	return lit
}

// SegmentedBar is a gauge of count rounded segments across the width, the
// first SegmentsLit of them in tone and the others on the track color.
func (th *Theme) SegmentedBar(gtx C, percent, count int, tone color.NRGBA) D {
	h := gtx.Dp(Space(1.5))
	gap := gtx.Dp(segmentedGap)
	width := gtx.Constraints.Max.X
	seg := max((width-gap*(count-1))/count, 1)
	lit := SegmentsLit(percent, count)
	track := th.Over(th.Border)
	for i := range count {
		c := track
		if i < lit {
			c = tone
		}
		x := i * (seg + gap)
		paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rectangle{Min: image.Pt(x, 0), Max: image.Pt(x+seg, h)}, h/2).Op(gtx.Ops))
	}
	return D{Size: image.Pt(count*seg+(count-1)*gap, h)}
}

// A WaveBar's wave, the mockup's: each segment lights 80ms after the one
// before, rising over 12% of a 1.6s lap and fading by 30% of it, so the lit
// head sweeps across, rests, and starts over.
const (
	waveLap  = 1600 * time.Millisecond
	waveStep = 80 * time.Millisecond
	waveRise = 0.12
	waveFall = 0.30
)

// WaveLevel is how lit segment i of a WaveBar is at now, 0 on the track and 1
// in full tone.
func WaveLevel(now time.Time, i int) float64 {
	at := (time.Duration(now.UnixNano()) - time.Duration(i)*waveStep) % waveLap
	if at < 0 {
		at += waveLap
	}
	phase := float64(at) / float64(waveLap)
	switch {
	case phase < waveRise:
		return phase / waveRise
	case phase < waveFall:
		return 1 - (phase-waveRise)/(waveFall-waveRise)
	}
	return 0
}

// WaveBar is a SegmentedBar for work with no share to tell: never filling,
// a lit wave sweeps its segments toward tone. It asks for the next frame while
// it is drawn.
func (th *Theme) WaveBar(gtx C, count int, tone color.NRGBA) D {
	h := gtx.Dp(Space(1.5))
	gap := gtx.Dp(segmentedGap)
	seg := max((gtx.Constraints.Max.X-gap*(count-1))/count, 1)
	track := th.Over(th.Border)
	for i := range count {
		c := MixOKLab(track, tone, WaveLevel(gtx.Now, i))
		x := i * (seg + gap)
		paint.FillShape(gtx.Ops, c, clip.UniformRRect(image.Rectangle{Min: image.Pt(x, 0), Max: image.Pt(x+seg, h)}, h/2).Op(gtx.Ops))
	}
	th.Loop(gtx)
	return D{Size: image.Pt(count*seg+(count-1)*gap, h)}
}
