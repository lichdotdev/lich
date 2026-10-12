package main

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n
// (settings.notificationsSettings).
const (
	groupWindow        = "Window"
	groupNotifications = "Notifications"

	needsInputTitle = "Notify me when a session needs input"
	needsInputDesc  = "Raise a desktop notification when a session is blocked on you — a permission prompt, a question — and the lich window is not the one you are in. While you are in lich nothing changes: the card's bell and the toast do the telling."
	finishedTitle   = "Notify me when a session finishes working"
	finishedDesc    = "Raise a desktop notification when a session ends its turn with nothing left to ask you, and the lich window is not the one you are in. Independent of the setting above — a long run you walked away from is worth knowing about even if you would rather not hear about every prompt. No toast comes with it: inside lich, the card and its project tab already say so."
	notifyAlso      = "desktop notification alert bell"
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
		sw, desc = &s.general.needsInput, needsInputDesc
		if sw.Update(gtx) {
			s.notify.setAttention(sw.Value)
		}
		sw.Value = prefs.attention == optInYes
	case finishedNotifyKey:
		sw, desc = &s.general.finished, finishedDesc
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
