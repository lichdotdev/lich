//go:build !darwin

package gioterm

import "gioui.org/io/key"

func altLetterCases() []keyCase {
	return []keyCase{
		{"alt+a", key.Event{Name: "A", Modifiers: key.ModAlt}, "\x1ba"},
		{"alt+shift+a", key.Event{Name: "A", Modifiers: key.ModAlt | key.ModShift}, "\x1bA"},
		{"alt+.", key.Event{Name: ".", Modifiers: key.ModAlt}, "\x1b."},
	}
}
