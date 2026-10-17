package main

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/ui"
)

// SettingGroup's pt-8 above a group past the first, and its mb-1 under the
// heading.
var (
	groupGapAbove = ui.Space(8)
	groupGapBelow = ui.Space(1)
)

// generalSwitches are the Notifications group's two switches.
type generalSwitches struct {
	needsInput ui.SwitchState
	finished   ui.SwitchState
}

// notifyRow answers the description and the switch of a Notifications row,
// false for any other key. The needs-input switch reads on only once the
// opt-in is answered yes, and turning it either way is itself the answer.
func (s *settingsScreen) notifyRow(gtx C, key string) (string, func(gtx C, th *ui.Theme) D, bool) {
	prefs, _ := s.notify.snapshot()
	var sw *ui.SwitchState
	var desc string
	switch key {
	case desktopNotifyKey:
		sw, desc = &s.general.needsInput, i18n.T("settings.notificationsSettings.needsInputDescription")
		if sw.Update(gtx) {
			s.notify.setAttention(sw.Value)
		}
		sw.Value = prefs.attention == optInYes
	case finishedNotifyKey:
		sw, desc = &s.general.finished, i18n.T("settings.notificationsSettings.finishedDescription")
		if sw.Update(gtx) {
			s.notify.setFinishedTurn(sw.Value)
		}
		sw.Value = prefs.finishedTurn
	default:
		return "", nil, false
	}
	return desc, func(gtx C, th *ui.Theme) D { return th.Switch(sw).Layout(gtx) }, true
}

// groupHeading is SettingGroup's label: small, muted and uppercase, the gap
// above it dropped for the section's first group.
func groupHeading(th *ui.Theme, label string, first bool) layout.Widget {
	inset := layout.Inset{Left: rowPadX, Top: groupGapAbove, Bottom: groupGapBelow}
	if first {
		inset.Top = 0
	}
	return func(gtx C) D {
		return inset.Layout(gtx, th.Text(ui.TextXS, strings.ToUpper(label)).Muted().Weight(font.SemiBold).Layout)
	}
}
