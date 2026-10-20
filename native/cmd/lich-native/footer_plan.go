package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icGauge     = icons.Lucide("gauge")
	icAhead     = icons.Lucide("trending-up")
	planMinWide = ui.Space(52)
	accountSize = ui.TextXS - 1
	// accountLeading is a 4/3 line height, at 11px.
	accountLeading unit.Sp = 14.67
)

// planDetails is the plan quota's tooltip body: a stacked gauge per window, its
// lock reason under it, and the account the reading was taken against.
//
// ponytail: the pace mark and the Locked reading carry no tooltip of their
// own here: a tooltip's content takes no hover in this window. The lock
// reason is spelled out under its gauge either way.
func planDetails(th *ui.Theme, plan lichclient.QuotaPlan, now time.Time) layout.Widget {
	var rows []layout.Widget
	for _, g := range quotaGauges(plan, now) {
		rows = append(rows, quotaGaugeRow(th, g))
	}
	if account := accountLine(plan); account != "" {
		rows = append(rows, accountRow(th, account))
	}
	return func(gtx C) D {
		gtx.Constraints.Min.X = min(gtx.Dp(planMinWide), gtx.Constraints.Max.X)
		return stack(gtx, 2.5, rows)
	}
}

// quotaGaugeRow is one gauge stacked, in the window's tone: the label (and pace
// mark) over the bar, the reading and time left on the right.
func quotaGaugeRow(th *ui.Theme, g quotaGauge) layout.Widget {
	tone := toneColor(th, g.Tone)
	reading := g.Reading
	if g.Left != "" {
		reading += " · " + g.Left
	}
	head := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Baseline, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return gaugeLabel(gtx, th, g) }),
			ui.Gap(3),
			layout.Rigid(th.Text(ui.TextXS, reading).In(tone).Layout),
		)
	}
	rows := []layout.Widget{head, func(gtx C) D { return gaugeBar(gtx, th, g.Percent, tone) }}
	gauge := func(gtx C) D { return stack(gtx, 1, rows) }
	if g.Locked == "" {
		return gauge
	}
	reason := para(th.Text(ui.TextXS, g.Locked).In(th.Destructive)).Layout
	return func(gtx C) D { return stack(gtx, 1, []layout.Widget{gauge, reason}) }
}

func gaugeLabel(gtx C, th *ui.Theme, g quotaGauge) D {
	children := []layout.FlexChild{layout.Flexed(1, th.Text(ui.TextXS, g.Label).Muted().Layout)}
	if g.Ahead {
		children = []layout.FlexChild{
			layout.Rigid(th.Text(ui.TextXS, g.Label).Muted().Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return icAhead.Layout(gtx, ui.Space(3), th.MutedForeground) }),
		}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// gaugeBar is a rounded track in the muted color, filled to percent in the
// window's tone.
func gaugeBar(gtx C, th *ui.Theme, percent int, tone color.NRGBA) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(1.5)))
	radius := ui.Space(0.75)
	track := gtx
	track.Constraints.Min = size
	ui.Fill(track, th.Muted, radius)
	fill := gtx
	fill.Constraints.Min = image.Pt(size.X*min(max(percent, 0), 100)/100, size.Y)
	if fill.Constraints.Min.X > 0 {
		ui.Fill(fill, tone, radius)
	}
	return D{Size: size}
}

// accountRow is the monospace line naming the account, under a separator.
func accountRow(th *ui.Theme, account string) layout.Widget {
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Separator),
			ui.VGap(2),
			layout.Rigid(para(th.Text(accountSize, account).Leading(accountLeading).Muted().Mono()).Layout),
		)
	}
}
