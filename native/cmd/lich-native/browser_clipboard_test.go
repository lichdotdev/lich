package main

import (
	"testing"

	"gioui.org/io/key"
)

func TestBrowserClipboardChordsAreTheBrowsersNotTheTerminals(t *testing.T) {
	cases := []struct {
		e    key.Event
		goos string
		want string
	}{
		{key.Event{Name: "V", Modifiers: key.ModCtrl, State: key.Press}, "linux", "paste"},
		{key.Event{Name: "V", Modifiers: key.ModCtrl | key.ModShift, State: key.Press}, "linux", "paste"},
		{key.Event{Name: "C", Modifiers: key.ModCtrl, State: key.Press}, "linux", "copy"},
		{key.Event{Name: "X", Modifiers: key.ModCtrl, State: key.Press}, "windows", "cut"},
		{key.Event{Name: "V", Modifiers: key.ModCommand, State: key.Press}, "darwin", "paste"},
		{key.Event{Name: "C", Modifiers: key.ModCommand, State: key.Press}, "darwin", "copy"},
		{key.Event{Name: "C", Modifiers: key.ModCtrl, State: key.Press}, "darwin", ""},
		{key.Event{Name: "V", Modifiers: key.ModCtrl, State: key.Release}, "linux", ""},
		{key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press}, "linux", ""},
	}
	for _, c := range cases {
		if got := browserClipboardChord(c.e, c.goos); got != c.want {
			t.Errorf("%v on %s: %q, want %q", c.e, c.goos, got, c.want)
		}
	}
}
