package main

import (
	"strings"

	"gioui.org/layout"

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
	blocks := append([]layout.Widget{th.Text(ui.TextXS, plan.Plan).Muted().Layout}, usageBlocks(th, plan, gtx.Now, nil)...)
	return ui.Stack(usageBlockGap, blocks...)(gtx)
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
