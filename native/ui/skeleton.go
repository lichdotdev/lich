package ui

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

const (
	// tabsPulsePeriod and tabsPulseTrough are Tailwind's animate-pulse:
	// `pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite`, where the keyframe
	// at 50% is `opacity: .5`.
	tabsPulsePeriod = 2 * time.Second
	tabsPulseTrough = 0.5
	// tabsBisections pin the bezier's parameter to well under a pixel of
	// opacity.
	tabsBisections = 24
)

// SkeletonStyle is shadcn's Skeleton: a pulsing bg-foreground/20 rounded-md
// block, the pulse an opacity over the whole element. Width and Height are the
// w-* and h-*; a zero Width is a block div's auto width, the whole row. A
// width past the constraint is clamped to it, as max-w-full does.
type SkeletonStyle struct {
	Width, Height unit.Dp
	theme         *Theme
}

// Skeleton is a placeholder of width by height.
func (th *Theme) Skeleton(width, height unit.Dp) SkeletonStyle {
	return SkeletonStyle{Width: width, Height: height, theme: th}
}

// Layout draws the block and asks for the next frame. Every skeleton pulses
// off the same clock, so a stack laid out together stays in step, as the CSS
// animations of elements mounted together do.
func (s SkeletonStyle) Layout(gtx C) D {
	w := gtx.Constraints.Max.X
	if s.Width > 0 {
		w = min(w, gtx.Dp(s.Width))
	}
	// An explicit size wins over Min, as w-* and h-* do on a block.
	size := image.Pt(w, min(gtx.Dp(s.Height), gtx.Constraints.Max.Y))
	gtx.Constraints.Min = size
	phase := float64(gtx.Now.UnixNano()%int64(tabsPulsePeriod)) / float64(tabsPulsePeriod)
	fill := Alpha(Alpha(s.theme.Foreground, 0.2), float32(tabsPulseOpacity(phase)))
	Fill(gtx, s.theme.Over(fill), RadiusMD)
	gtx.Execute(op.InvalidateCmd{})
	return layout.Dimensions{Size: size}
}

// tabsPulseOpacity is the keyframes at phase 0..1 of the period: full at
// both ends, the trough at the middle, each half eased on its own.
func tabsPulseOpacity(phase float64) float64 {
	if phase < 0.5 {
		return 1 - (1-tabsPulseTrough)*tabsEase(phase*2)
	}
	return tabsPulseTrough + (1-tabsPulseTrough)*tabsEase((phase-0.5)*2)
}

// tabsEase is cubic-bezier(0.4, 0, 0.6, 1): the curve's x is solved for the
// parameter by bisection, then read off at y.
func tabsEase(progress float64) float64 {
	const x1, y1, x2, y2 = 0.4, 0, 0.6, 1
	at := func(s, a, b float64) float64 {
		u := 1 - s
		return 3*u*u*s*a + 3*u*s*s*b + s*s*s
	}
	lo, hi := 0.0, 1.0
	for range tabsBisections {
		mid := (lo + hi) / 2
		if at(mid, x1, x2) < progress {
			lo = mid
		} else {
			hi = mid
		}
	}
	return at((lo+hi)/2, y1, y2)
}

// SkeletonBarStyle is a bar of Width by Height centred in a line of height
// Line.
type SkeletonBarStyle struct {
	Width, Line, Height unit.Dp
	theme               *Theme
}

func (th *Theme) SkeletonBar(width, line, height unit.Dp) SkeletonBarStyle {
	return SkeletonBarStyle{Width: width, Line: line, Height: height, theme: th}
}

func (b SkeletonBarStyle) Layout(gtx C) D {
	pad := (b.Line - b.Height) / 2
	return layout.Inset{Top: pad, Bottom: pad}.Layout(gtx, b.theme.Skeleton(b.Width, b.Height).Layout)
}

// SkeletonTextStyle is a bar of Width standing for a line of text at Size: as
// tall as the font, centred in the line the text takes.
type SkeletonTextStyle struct {
	Size  unit.Sp
	Width unit.Dp
	theme *Theme
}

func (th *Theme) SkeletonText(size unit.Sp, width unit.Dp) SkeletonTextStyle {
	return SkeletonTextStyle{Size: size, Width: width, theme: th}
}

func (t SkeletonTextStyle) Layout(gtx C) D {
	line := measured(gtx, t.theme.Text(t.Size, "Ag").Layout).Y
	bar := min(gtx.Sp(t.Size), line)
	defer op.Offset(image.Pt(0, (line-bar)/2)).Push(gtx.Ops).Pop()
	t.theme.Skeleton(t.Width, gtx.Metric.PxToDp(bar)).Layout(gtx)
	return D{Size: image.Pt(min(gtx.Dp(t.Width), gtx.Constraints.Max.X), line)}
}

// SkeletonShapeStyle is a block the size Widget would take: a button whose
// label is known but whose state is not.
type SkeletonShapeStyle struct {
	Widget layout.Widget
	theme  *Theme
}

func (th *Theme) SkeletonShape(w layout.Widget) SkeletonShapeStyle {
	return SkeletonShapeStyle{Widget: w, theme: th}
}

func (s SkeletonShapeStyle) Layout(gtx C) D {
	size := measured(gtx, s.Widget)
	return s.theme.Skeleton(gtx.Metric.PxToDp(size.X), gtx.Metric.PxToDp(size.Y)).Layout(gtx)
}

// SkeletonFill lays row after row down to the bottom of gtx, as a loaded list
// fills its panel. MaxRows bounds it under an unbounded height, PadY insets
// the rows top and bottom.
type SkeletonFill struct {
	MaxRows int
	PadY    unit.Dp
}

func (f SkeletonFill) Layout(gtx C, row func(gtx C, i int) D) D {
	return layout.Inset{Top: f.PadY, Bottom: f.PadY}.Layout(gtx, func(gtx C) D {
		defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
		y := 0
		for i := 0; y < gtx.Constraints.Max.Y && i < f.MaxRows; i++ {
			g := gtx
			g.Constraints.Min.Y = 0
			stack := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
			y += row(g, i).Size.Y
			stack.Pop()
		}
		return D{Size: image.Pt(gtx.Constraints.Max.X, min(y, gtx.Constraints.Max.Y))}
	})
}

// SkeletonRow is a full-width list row's padding around its bars.
type SkeletonRow struct {
	PadX, PadY unit.Dp
}

func (r SkeletonRow) Layout(gtx C, content layout.Widget) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Inset{Left: r.PadX, Right: r.PadX, Top: r.PadY, Bottom: r.PadY}.Layout(gtx, content)
}

// measured is the size w takes, laid out and thrown away.
func measured(gtx C, w layout.Widget) image.Point {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	size := w(gtx).Size
	macro.Stop()
	return size
}
