package main

import "github.com/omartelo/lich/native/ui"

// ponytail: English until the native window has i18n (settings.windowSetting).
const (
	windowKindTitle = "Window"
	windowKindAlso  = "web native experimental window open"
	windowKindDesc  = "Which window lich opens. The native window is new and still experimental; if it fails to start, lich opens the web window instead."
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

// windowKindOf is the stored choice, the web window for nothing stored or a
// value from a newer lich.
func windowKindOf(stored string) string {
	if stored == windowNative {
		return windowNative
	}
	return windowWeb
}

// windowKindControl is WindowSetting's toggle; a change takes effect the next
// time lich opens its window.
func (s *settingsScreen) windowKindControl(gtx C, th *ui.Theme) D {
	g := &s.look.windowTo
	if g.Update(gtx) {
		p := s.look.prefs
		p.windowKind = windowKindOf(g.Value)
		s.commitAppearance(p, windowKindKey, p.windowKind)
	}
	g.Value = s.look.prefs.windowKind
	return th.ToggleGroup(g, windowKindChoices...).Layout(gtx)
}
