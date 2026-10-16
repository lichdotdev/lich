package main

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (settings.windowSetting).
const (
	windowKindTitle = "Window"
	windowKindAlso  = "web native experimental window open"
	windowKindDesc  = "Which window lich opens. The native window is new and still experimental; if it fails to start, lich opens the web window instead."
	switchTitle     = "Switch to the %s window now?"
	switchDesc      = "This window closes and the %s one opens. Your sessions keep running."
	switchNow       = "Switch now"
	switchLater     = "Later"
	switchFailed    = "Could not open the %s window: %v"
	windowNameWeb   = "web"
)

// system.WindowWeb and system.WindowNative.
const (
	windowWeb    = "web"
	windowNative = "native"
)

var windowKindChoices = []ui.ToggleItem{
	{Value: windowWeb, Text: "Web"},
	{Value: windowNative, Text: "Native (experimental)"},
}

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
	return th.ToggleGroup(g, windowKindChoices...).Layout(gtx)
}

// layoutWindowSwitch draws the switch offer while it is open.
func (s *settingsScreen) layoutWindowSwitch(gtx C, th *ui.Theme) {
	w := &s.switcher
	if w.now.Clicked(gtx) && w.open {
		w.open = false
		go func() {
			if err := s.client.OpenWindow(s.ctx, windowWeb); err != nil {
				s.theme.toaster.Show(failure(switchFailed, windowNameWeb, err))
			}
		}()
	}
	if w.later.Clicked(gtx) {
		w.open = false
	}
	on := th.On(th.Popover)
	later := on.Button(&w.later, switchLater)
	later.Variant = ui.VariantGhost
	now := on.Button(&w.now, switchNow)
	dlg := th.Dialog(&w.dialog, w.open, fmt.Sprintf(switchTitle, windowNameWeb), fmt.Sprintf(switchDesc, windowNameWeb))
	dlg.Actions = []layout.Widget{later.Layout, now.Layout}
	if dlg.Layout(gtx) {
		w.open = false
	}
}
