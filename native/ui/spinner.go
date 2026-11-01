package ui

import (
	"image"
	"image/color"
	"math"
	"sync/atomic"
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

// LoopFrame is how often a looping animation (a spinner, the busy ring, a
// running service's beat) redraws. Every frame repaints the whole window, so
// this is the window's rate while one runs: 24fps, film's rate, reads as
// smooth on a glyph, where the display's rate cost 60% of a laptop GPU
// (measured 2026-11-01).
const LoopFrame = 42 * time.Millisecond

// LoopBeat is when the next looping frame is due, shared by every Theme of a
// window: output that arrives before it waits for that frame instead of
// asking for one of its own.
type LoopBeat struct {
	due atomic.Int64
}

// Pending reports whether a looping frame is due after now.
func (b *LoopBeat) Pending(now time.Time) bool {
	return b != nil && b.due.Load() > now.UnixNano()
}

// Loop asks for a looping animation's next frame, LoopFrame from now, unless
// the window is Still: then the animation holds the frame it stands at.
func (th *Theme) Loop(gtx layout.Context) {
	if th.Still {
		return
	}
	at := gtx.Now.Add(LoopFrame)
	if th.Beat != nil {
		th.Beat.due.Store(at.UnixNano())
	}
	gtx.Execute(op.InvalidateCmd{At: at})
}

var icSpinner = icons.Lucide("loader-circle")

// BusyRing draws on Lucide's 24-unit grid: loader-circle's r=10 circle and
// stroke-width 2, under the sidebar busy ring's 25% track.
const (
	lucideViewBox      = 24
	lucideCircleRadius = 10
	lucideStroke       = 2
	ringTrackOpacity   = 0.25
)

// Spinner is LoaderCircle under animate-spin, size square, and asks for the
// next frame while it is drawn.
func (th *Theme) Spinner(gtx layout.Context, size unit.Dp, fg color.NRGBA) layout.Dimensions {
	turn := float64(gtx.Now.UnixNano()%int64(spinLap)) / float64(spinLap)
	half := float32(gtx.Dp(size)) / 2
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), float32(turn*2*math.Pi))).Push(gtx.Ops).Pop()
	th.Loop(gtx)
	return icSpinner.Layout(gtx, size, fg)
}

// RingTrack is the faint full circle a BusyRing of fg turns over, composited
// onto th's surface.
func (th *Theme) RingTrack(fg color.NRGBA) color.NRGBA {
	return th.Over(Alpha(fg, ringTrackOpacity))
}

// BusyRing is the busy ring at a Lucide circle's size and weight, so it lines up
// with the glyphs beside it: track all the way round under a quarter arc of
// fg turning once a lap. It asks for the next frame while it is drawn.
func (th *Theme) BusyRing(gtx layout.Context, size unit.Dp, fg, track color.NRGBA) layout.Dimensions {
	px := gtx.Dp(size)
	scale := float32(px) / lucideViewBox
	center := f32.Pt(float32(px)/2, float32(px)/2)
	radius, width := lucideCircleRadius*scale, lucideStroke*scale
	StrokeArc(gtx, center, radius, width, 0, 2*math.Pi, track)
	turn := float32(gtx.Now.UnixNano()%int64(spinLap)) / float32(spinLap) * 2 * math.Pi
	StrokeArc(gtx, center, radius, width, turn, math.Pi/2, fg)
	th.Loop(gtx)
	return layout.Dimensions{Size: image.Pt(px, px)}
}

// StrokeArc strokes width wide the arc of radius around center that starts
// at the angle start, in radians clockwise from 3 o'clock, and runs sweep.
// The path is recorded once per distinct arc and kept, so a ring drawn the
// same every frame keeps its vertices on the GPU; see Icon.
func StrokeArc(gtx layout.Context, center f32.Point, radius, width, start, sweep float32, c color.NRGBA) {
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: arcs.get(arcKey{center, radius, start, sweep}), Width: width}.Op())
}

type arcKey struct {
	center        f32.Point
	radius, start float32
	sweep         float32
}

// arcPaths is StrokeArc's record of paths. A turning arc is a new path every
// frame, so the record is bounded: past arcPathsMax paths it starts over in a
// fresh op.Ops, and the old one lives on for as long as a frame still calls
// into it.
var arcs = arcPaths{paths: map[arcKey]clip.PathSpec{}}

const arcPathsMax = 1024

type arcPaths struct {
	ops   *op.Ops
	paths map[arcKey]clip.PathSpec
}

func (a *arcPaths) get(k arcKey) clip.PathSpec {
	if spec, ok := a.paths[k]; ok {
		return spec
	}
	if a.ops == nil || len(a.paths) >= arcPathsMax {
		a.ops = new(op.Ops)
		clear(a.paths)
	}
	var p clip.Path
	p.Begin(a.ops)
	p.MoveTo(f32.Pt(k.center.X+k.radius*float32(math.Cos(float64(k.start))), k.center.Y+k.radius*float32(math.Sin(float64(k.start)))))
	p.ArcTo(k.center, k.center, k.sweep)
	spec := p.End()
	a.paths[k] = spec
	return spec
}
