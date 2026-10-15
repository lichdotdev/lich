package ui

import (
	"image"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const (
	// progressHeight is the update toast's h-1.5 bar (UpdateProgressToast.tsx).
	progressHeight = 1.5
	// progressSlide is index.css's update-indeterminate: a third of the track
	// sliding from translateX(-100%) to translateX(300%) every 1.4s.
	progressSlide      = 1400 * time.Millisecond
	progressSlideShare = 3
	progressSlideFrom  = -1
	progressSlideTo    = 3
)

// ProgressStyle is the update toast's download bar: a bg-muted rounded-full
// track, filled in the foreground to Percent, or crossed by a sliding third
// while Percent is negative (the total is unknown).
type ProgressStyle struct {
	Percent int
	theme   *Theme
}

// Progress is a bar at percent, 0 to 100; a negative percent is
// indeterminate.
func (th *Theme) Progress(percent int) ProgressStyle {
	return ProgressStyle{Percent: percent, theme: th}
}

// Layout draws the bar the width of the row; an indeterminate one asks for
// the next frame.
func (p ProgressStyle) Layout(gtx C) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(Space(progressHeight)))
	track := clip.UniformRRect(image.Rectangle{Max: size}, size.Y/2)
	paint.FillShape(gtx.Ops, p.theme.Muted, track.Op(gtx.Ops))
	defer track.Push(gtx.Ops).Pop()
	bar := image.Rectangle{Max: image.Pt(size.X*min(p.Percent, 100)/100, size.Y)}
	if p.Percent < 0 {
		width := size.X / progressSlideShare
		phase := float64(gtx.Now.UnixNano()%int64(progressSlide)) / float64(progressSlide)
		x := int(float64(width) * (progressSlideFrom + (progressSlideTo-progressSlideFrom)*tabsEase(phase)))
		bar = image.Rect(x, 0, x+width, size.Y)
		gtx.Execute(op.InvalidateCmd{})
	}
	paint.FillShape(gtx.Ops, p.theme.Foreground, clip.UniformRRect(bar, size.Y/2).Op(gtx.Ops))
	return D{Size: size}
}
