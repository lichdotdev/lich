package gioterm

import "gioui.org/io/key"

// On macOS Option+letter types what Cocoa composed, through the EditEvent.
func altLetterCases() []keyCase {
	return []keyCase{
		{"alt+a", key.Event{Name: "A", Modifiers: key.ModAlt}, ""},
		{"alt+shift+a", key.Event{Name: "A", Modifiers: key.ModAlt | key.ModShift}, ""},
		{"alt+.", key.Event{Name: ".", Modifiers: key.ModAlt}, ""},
	}
}
