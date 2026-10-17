package main

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// system.WindowWeb and system.WindowNative.
const (
	windowWeb    = "web"
	windowNative = "native"
)

// windowSwitch is the dialog that offers to switch to the web window at once
// after it was chosen here.
type windowSwitch struct {
	open       bool
	dialog     ui.DialogState
	now, later widget.Clickable
}

// windowKindOf is the stored choice, the web window for nothing stored or a
// value from a newer lich.
func windowKindOf(stored string) string {
	if stored == windowNative {
		return windowNative
	}
	return windowWeb
}

// windowKindControl is WindowSetting's toggle. Choosing the web window from
// this one offers to switch now; otherwise the choice waits for the next time
// lich opens its window.
func (s *settingsScreen) windowKindControl(gtx C, th *ui.Theme) D {
	g := &s.look.windowTo
	if g.Update(gtx) {
		p := s.look.prefs
		p.windowKind = windowKindOf(g.Value)
		s.commitAppearance(p, windowKindKey, p.windowKind)
		s.switcher.open = p.windowKind == windowWeb
	}
	g.Value = s.look.prefs.windowKind
	choices := []ui.ToggleItem{
		{Value: windowWeb, Text: i18n.T("settings.windowSetting.choice.web.label")},
		{Value: windowNative, Text: i18n.T("settings.windowSetting.choice.native.label")},
	}
	return th.ToggleGroup(g, choices...).Layout(gtx)
}

// layoutWindowSwitch draws the switch offer while it is open.
func (s *settingsScreen) layoutWindowSwitch(gtx C, th *ui.Theme) {
	w := &s.switcher
	if w.now.Clicked(gtx) && w.open {
		w.open = false
		go func() {
			if err := s.client.OpenWindow(s.ctx, windowWeb); err != nil {
				title := i18n.T("settings.settingsWindow.switchFailed", i18n.Args{"error": lichclient.ErrorText(err)})
				s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: title})
			}
		}()
	}
	if w.later.Clicked(gtx) {
		w.open = false
	}
	on := th.On(th.Popover)
	later := on.Button(&w.later, i18n.T("settings.windowSetting.switch.later"))
	later.Variant = ui.VariantGhost
	now := on.Button(&w.now, i18n.T("settings.windowSetting.switch.now"))
	dlg := th.Dialog(&w.dialog, w.open, i18n.T("settings.settingsWindow.switchTitle"), i18n.T("settings.settingsWindow.switchDescription"))
	dlg.Actions = []layout.Widget{later.Layout, now.Layout}
	if dlg.Layout(gtx) {
		w.open = false
	}
}
