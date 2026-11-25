package main

import (
	"fmt"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// planBlock is PlanUsageSetting: what is left of the provider's subscription,
// the state of the provider above everything that configures it. It is a
// reading, not a setting, so it has no scope of its own. heading is the title
// it is drawn under, "settings.planUsageSetting.title" when "".
func (s *settingsScreen) planBlock(gtx C, th *ui.Theme, values scopedValues, plan backend.QuotaPlan, first bool, heading string) D {
	r := s.providerRow(gtx, settingEntry{titleKey: "settings.planUsageSetting.title"}, values)
	r.kind, r.overriddenIn = rowPlain, nil
	if heading != "" {
		r.title = heading
	}
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D { return planBody(gtx, th, plan) })
}

func planBody(gtx C, th *ui.Theme, plan backend.QuotaPlan) D {
	switch plan.Status {
	case "signed-out":
		return planSignedOutLine(gtx, th, loginCommands[plan.Provider])
	case "error":
		return th.Text(ui.TextXS, i18n.T("settings.planUsageSetting.readFailed")).Muted().Wrapped().Layout(gtx)
	}
	lines := []layout.Widget{th.Text(ui.TextXS, plan.Plan).Muted().Layout}
	for _, g := range quotaGauges(rankedWindows(plan), gtx.Now) {
		lines = append(lines, planLine(th, g))
	}
	if plan.Extra != nil {
		lines = append(lines, extraBlock(th, *plan.Extra, i18n.Current()))
	}
	if resets := liveResets(plan.Resets, gtx.Now); len(resets) > 0 {
		lines = append(lines, resetsLine(th, resets))
	}
	if plan.ResetCredits > 0 {
		lines = append(lines, tipRow(th, contextRow{
			label: i18n.T("shell.planQuota.resetCredits"),
			value: i18n.T("shell.planQuota.resetCreditsAvailable", i18n.Args{"count": plan.ResetCredits}),
		}))
	}
	return ui.Stack(planLineGap, lines...)(gtx)
}

// A plan window's line: its name, the bar, the share and when it resets, each
// in a column of its own so the windows' bars start and end together.
var (
	planLabelW   = ui.Space(28)
	planReadingW = ui.Space(12)
	planWhenW    = ui.Space(36)
)

// planLineGap is the Space step between a plan's lines.
const planLineGap = 2

// planLine is one window on a line. A lock or a pace that empties the window
// first takes the reset's place, in their tones.
func planLine(th *ui.Theme, g quotaGauge) layout.Widget {
	bar := contextBarColor(th, g.Percent)
	when, tone := "", th.MutedForeground
	switch {
	case g.Locked != "":
		bar, when, tone = th.Destructive, g.Locked, th.Destructive
	case g.RunsOut != "":
		when, tone = i18n.T("shell.planQuota.runsOut", i18n.Args{"time": g.RunsOut}), toneColor(th, toneWait)
	case g.Left != "":
		when = i18n.T("shell.planQuota.resetsIn", i18n.Args{"time": g.Left})
	}
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(planColumn(planLabelW, layout.W, th.Text(ui.TextXS, g.Label).Weight(font.Medium).Layout)),
			layout.Flexed(1, func(gtx C) D { return th.SegmentedBar(gtx, g.Percent, contextSegments, bar) }),
			layout.Rigid(planColumn(planReadingW, layout.E, th.Text(ui.TextXS, g.Reading).Mono().Layout)),
			ui.Gap(3),
			layout.Rigid(planColumn(planWhenW, layout.W, th.Text(ui.TextXS, when).In(tone).Layout)),
		)
	}
}

func planColumn(w unit.Dp, side layout.Direction, content layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(w), gtx.Dp(w)
		return side.Layout(gtx, content)
	}
}

// resetsLine is the grants on one line: how many are left, then each with
// its count and when it ends.
func resetsLine(th *ui.Theme, resets []backend.QuotaReset) layout.Widget {
	total := 0
	parts := make([]string, 0, len(resets)+1)
	for _, r := range resets {
		total += r.Left
	}
	parts = append(parts, i18n.T("shell.planQuota.resetsLeft", i18n.Args{"count": total}))
	for _, r := range resets {
		until, _ := time.Parse(time.RFC3339, r.EndsAt)
		parts = append(parts, fmt.Sprintf("%s ×%d %s", resetName(r), r.Left, shortUntil(until)))
	}
	return func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, i18n.T("shell.planQuota.resets")).Weight(font.Medium).Layout),
			ui.Gap(2),
			layout.Flexed(1, th.Text(ui.TextXS, strings.Join(parts, profileAccountSplit)).Muted().Wrapped().Layout),
		)
	}
}

// commandSlot stands in for the command while the sentence is split around
// it, so the command can be drawn in mono between its two halves.
const commandSlot = "\x00"

// planSignedOutLine says how the login is redone, the command in mono: lich
// reads the credentials that command writes and never refreshes them itself.
func planSignedOutLine(gtx C, th *ui.Theme, command string) D {
	if command == "" {
		return th.Text(ui.TextXS, i18n.T("settings.planUsageSetting.signedOut")).Muted().Layout(gtx)
	}
	lead, tail, _ := strings.Cut(i18n.T("settings.planUsageSetting.signedOutRun", i18n.Args{"command": commandSlot}), commandSlot)
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, lead).Muted().Layout),
		layout.Rigid(th.Text(ui.TextXS, command).Mono().Layout),
		layout.Rigid(th.Text(ui.TextXS, tail).Muted().Layout),
	)
}
