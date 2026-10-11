package ui

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Chip classes of SessionPhaseChips: `inline-flex items-baseline gap-1
// rounded-md px-1.5 py-0.5 text-xs`, the count `font-mono text-[0.625rem]
// opacity-70`.
const (
	chipCountSize    unit.Sp = 10
	chipCountOpacity         = 0.7
	chipEmptyOpacity         = 0.4 // !on && count === 0 && "opacity-40"
	chipHoverFill            = 0.5 // hover:bg-accent/50
	chipRingWidth    unit.Dp = 2   // focus-visible:ring-2 ring-ring
)

var (
	chipPadX = Space(1.5)
	chipPadY = Space(0.5)
	chipGap  = Space(1)
)

// ChipStyle is SessionPhaseChips' Chip: a filter toggle with the count of
// what it would show. A click is the caller's to read from Click, and On is
// the caller's aria-pressed, so a set of chips can be several at once or
// cleared by an All chip. Clicking one takes no focus, as the web's
// preventDefault on mousedown keeps it in the filter field.
type ChipStyle struct {
	Click *widget.Clickable
	Label string
	Count int
	On    bool
	theme *Theme
}

func (th *Theme) Chip(click *widget.Clickable, label string, count int, on bool) ChipStyle {
	return ChipStyle{Click: click, Label: label, Count: count, On: on, theme: th}
}

func (c ChipStyle) Layout(gtx C) D {
	m := op.Record(gtx.Ops)
	d := c.Click.Layout(gtx, func(gtx C) D { return c.draw(gtx, c.Click.Hovered()) })
	call := m.Stop()
	if gtx.Focused(c.Click) {
		toggleStroke(gtx, d.Size, gtx.Dp(chipRingWidth), gtx.Dp(RadiusMD), c.theme.Ring)
	}
	call.Add(gtx.Ops)
	return d
}

// colors are the chip's classes; the dimming of an empty chip fades the
// whole button, so it lands on every color of it.
func (c ChipStyle) colors(hovered bool) (bg, fg color.NRGBA) {
	th := c.theme
	switch {
	case c.On:
		bg, fg = th.Accent, th.AccentForeground
	case hovered:
		bg, fg = th.Over(Alpha(th.Accent, chipHoverFill)), th.MutedForeground
	default:
		fg = th.MutedForeground
	}
	if !c.On && c.Count == 0 {
		fade := func(x color.NRGBA) color.NRGBA {
			if x.A == 0 {
				return x
			}
			return Over(th.Surface, Alpha(x, chipEmptyOpacity))
		}
		bg, fg = fade(bg), fade(fg)
	}
	return bg, fg
}

func (c ChipStyle) draw(gtx C, hovered bool) D {
	bg, fg := c.colors(hovered)
	on := c.theme
	if bg.A != 0 {
		on = c.theme.On(bg)
	}
	count := on.Text(chipCountSize, strconv.Itoa(c.Count)).Mono().In(on.Over(Alpha(fg, chipCountOpacity)))
	gtx.Constraints.Min = image.Point{}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return Fill(gtx, bg, RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: chipPadX, Right: chipPadX, Top: chipPadY, Bottom: chipPadY}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(on.Text(TextXS, c.Label).In(fg).Layout),
					layout.Rigid(layout.Spacer{Width: chipGap}.Layout),
					layout.Rigid(count.Layout),
				)
			})
		},
	)
}
