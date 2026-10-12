package main

import (
	"image"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (settings.planUsageSetting).
const (
	planSignedOut    = "Signed out."
	planSignedOutRun = "Signed out. Run %s to read plan usage."
	planReadFailed   = "Could not read plan usage. The provider's usage endpoint is undocumented and may have changed."
	planUsed         = "used"
	planResets       = "resets"
)

// The gauge grid's columns (gaugeGrid): every one fixed but the bar's, so
// gauges of the same thing end in the same place.
var (
	planLabelW   = ui.Space(28)
	planReadingW = ui.Space(10)
	planLeftW    = ui.Space(16)
	planHeadSize = unit.Sp(11) // text-2xs
)

// planBlock is PlanUsageSetting: what is left of the provider's subscription,
// the state of the provider above everything that configures it. It is a
// reading, not a setting, so it has no scope of its own. heading is the title
// it is drawn under, planTitle when "".
func (s *settingsScreen) planBlock(gtx C, th *ui.Theme, values scopedValues, plan lichclient.QuotaPlan, first bool, heading string) D {
	r := s.providerRow(gtx, settingEntry{title: planTitle}, values)
	r.kind, r.overriddenIn = rowPlain, nil
	r.description = planDesc
	if heading != "" {
		r.title = heading
	}
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D { return planBody(gtx, th, plan) })
}

func planBody(gtx C, th *ui.Theme, plan lichclient.QuotaPlan) D {
	switch plan.Status {
	case "signed-out":
		return planSignedOutLine(gtx, th, loginCommands[plan.Provider])
	case "error":
		return wrapped(gtx, th.Text(ui.TextXS, planReadFailed).Muted())
	}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(ui.TextXS)*proseCh*6/10)
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return planHead(gtx, th, plan.Plan) })}
	for _, g := range quotaGauges(plan, gtx.Now) {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return planGauge(gtx, th, g) }))
	}
	if account := accountLine(plan); account != "" {
		children = append(children, ui.VGap(2), layout.Rigid(th.Text(accountSize, account).Muted().Mono().Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// planSignedOutLine says how the login is redone, the command in mono: lich
// reads the credentials that command writes and never refreshes them itself.
func planSignedOutLine(gtx C, th *ui.Theme, command string) D {
	if command == "" {
		return th.Text(ui.TextXS, planSignedOut).Muted().Layout(gtx)
	}
	lead, tail, _ := strings.Cut(planSignedOutRun, "%s")
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, lead).Muted().Layout),
		layout.Rigid(th.Text(ui.TextXS, command).Mono().Layout),
		layout.Rigid(th.Text(ui.TextXS, tail).Muted().Layout),
	)
}

// planHead is the grid's header row: the plan doubles as the left header,
// which keeps it from floating above the rows as a line of its own.
func planHead(gtx C, th *ui.Theme, plan string) D {
	muted := func(text string) layout.Widget {
		return th.Text(planHeadSize, strings.ToUpper(text)).Muted().Layout
	}
	return planRow(gtx,
		th.Text(ui.TextXS, plan).Muted().Layout,
		func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} },
		muted(planUsed), muted(planResets))
}

// planGauge is QuotaGauge on one line: the label (and the pace mark), the
// bar, the reading and the time left, the row in the window's tone.
func planGauge(gtx C, th *ui.Theme, g quotaGauge) D {
	tone := toneColor(th, g.Tone)
	label := func(gtx C) D {
		children := []layout.FlexChild{layout.Flexed(1, th.Text(ui.TextXS, g.Label).Muted().Layout)}
		if g.Ahead {
			children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D { return icAhead.Layout(gtx, ui.Space(3), tone) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
	// The pane is drawn on the page's own color, which in the light theme is
	// the track's: the footer's bar would be a fill with nothing to fill.
	onPage := *th
	onPage.Muted = th.Over(th.Border)
	row := func(gtx C) D {
		return planRow(gtx, label,
			func(gtx C) D { return gaugeBar(gtx, &onPage, g.Percent, tone) },
			th.Text(ui.TextXS, g.Reading).In(tone).Layout,
			th.Text(ui.TextXS, g.Left).Muted().Layout)
	}
	if g.Locked == "" {
		return row(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(row),
		ui.VGap(1),
		layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, g.Locked).In(th.Destructive)) }),
	)
}

// planRow lays out the grid's four columns.
func planRow(gtx C, label, bar, reading, left layout.Widget) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		fixedColumn(planLabelW, label),
		ui.Gap(3),
		layout.Flexed(1, bar),
		ui.Gap(3),
		fixedColumn(planReadingW, alignEnd(reading)),
		ui.Gap(3),
		fixedColumn(planLeftW, alignEnd(left)),
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
