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

	"github.com/lichdotdev/lich/native/ui/icons"
)

// spinLap is animate-spin's: one turn a second.
const spinLap = time.Second

var icSpinner = icons.Lucide("loader-circle")

// Ring draws on Lucide's 24-unit grid: loader-circle's r=10 circle and
// stroke-width 2, under the sidebar busy ring's 25% track.
const (
	lucideViewBox      = 24
	lucideCircleRadius = 10
	lucideStroke       = 2
	ringTrackOpacity   = 0.25
)

// Spinner is LoaderCircle under animate-spin, size square, and asks for the
// next frame while it is drawn.
func Spinner(gtx layout.Context, size unit.Dp, fg color.NRGBA) layout.Dimensions {
	turn := float64(gtx.Now.UnixNano()%int64(spinLap)) / float64(spinLap)
	half := float32(gtx.Dp(size)) / 2
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), float32(turn*2*math.Pi))).Push(gtx.Ops).Pop()
	gtx.Execute(op.InvalidateCmd{})
	return icSpinner.Layout(gtx, size, fg)
}

// RingTrack is the faint full circle a Ring of fg turns over, composited
// onto th's surface.
func (th *Theme) RingTrack(fg color.NRGBA) color.NRGBA {
	return th.Over(Alpha(fg, ringTrackOpacity))
}

// Ring is the busy ring at a Lucide circle's size and weight, so it lines up
// with the glyphs beside it: track all the way round under a quarter arc of
// fg turning once a lap. It asks for the next frame while it is drawn.
func Ring(gtx layout.Context, size unit.Dp, fg, track color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	scale := float32(px) / lucideViewBox
	center := f32.Pt(float32(px)/2, float32(px)/2)
	radius, width := lucideCircleRadius*scale, lucideStroke*scale
	StrokeArc(gtx, center, radius, width, 0, 2*math.Pi, track)
	turn := float32(gtx.Now.UnixNano()%int64(spinLap)) / float32(spinLap) * 2 * math.Pi
	StrokeArc(gtx, center, radius, width, turn, math.Pi/2, fg)
	gtx.Execute(op.InvalidateCmd{})
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// StrokeArc strokes width wide the arc of radius around center that starts
// at the angle start, in radians clockwise from 3 o'clock, and runs sweep.
func StrokeArc(gtx layout.Context, center f32.Point, radius, width, start, sweep float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(center.X+radius*float32(math.Cos(float64(start))), center.Y+radius*float32(math.Sin(float64(start)))))
	p.ArcTo(center, center, sweep)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}
