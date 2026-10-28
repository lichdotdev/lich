package main

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

func TestBrowserKeyNamesWhatTypesNoText(t *testing.T) {
	cases := []struct {
		e    key.Event
		key  string
		code string
		kc   int
		mods int
		kind string
	}{
		{key.Event{Name: key.NameReturn, State: key.Press}, "Enter", "Enter", 13, 0, "down"},
		{key.Event{Name: key.NameDeleteBackward, State: key.Release}, "Backspace", "Backspace", 8, 0, "up"},
		{key.Event{Name: key.NameLeftArrow, Modifiers: key.ModShift, State: key.Press}, "ArrowLeft", "ArrowLeft", 37, 8, "down"},
		{key.Event{Name: "A", Modifiers: key.ModCtrl, State: key.Press}, "a", "KeyA", 65, 2, "down"},
		{key.Event{Name: "Z", Modifiers: key.ModCtrl | key.ModShift, State: key.Press}, "Z", "KeyZ", 90, 10, "down"},
		{key.Event{Name: "1", Modifiers: key.ModAlt, State: key.Press}, "1", "Digit1", 49, 1, "down"},
	}
	for _, c := range cases {
		got, ok := browserKey("s", c.e)
		if !ok || got.Key != c.key || got.Code != c.code || got.KeyCode != c.kc || got.Modifiers != c.mods || got.Type != c.kind || got.ID != "s" {
			t.Errorf("%v: got %+v, %v", c.e, got, ok)
		}
	}
}

func TestBrowserKeyLeavesTypedTextToTheEditEvent(t *testing.T) {
	if got, ok := browserKey("s", key.Event{Name: "A", State: key.Press}); ok {
		t.Fatalf("a plain letter became a key event: %+v", got)
	}
}

func TestBrowserButton(t *testing.T) {
	if browserButton(pointer.ButtonPrimary) != "left" || browserButton(pointer.ButtonSecondary) != "right" ||
		browserButton(pointer.ButtonTertiary) != "middle" || browserButton(0) != "" {
		t.Fatal("browserButton")
	}
}

func TestPageZoomChordStepsAndResets(t *testing.T) {
	press := func(name key.Name, mods key.Modifiers) key.Event {
		return key.Event{Name: name, Modifiers: mods, State: key.Press}
	}
	cases := []struct {
		e    key.Event
		goos string
		from float64
		want float64
		ok   bool
	}{
		{press("-", key.ModCtrl), "linux", 1, 0.9, true},
		{press("=", key.ModCtrl), "linux", 1, 1.1, true},
		{press("+", key.ModCtrl|key.ModShift), "windows", 1, 1.1, true},
		{press("0", key.ModCtrl), "linux", 0.7, 1, true},
		{press("-", key.ModCtrl), "linux", 0.5, 0.5, true},
		{press("=", key.ModCtrl), "linux", 2, 2, true},
		{press("-", key.ModCommand), "darwin", 1, 0.9, true},
		{press("-", key.ModCtrl), "darwin", 1, 1, false},
		{press("-", 0), "linux", 1, 1, false},
		{press("a", key.ModCtrl), "linux", 1, 1, false},
		{key.Event{Name: "-", Modifiers: key.ModCtrl, State: key.Release}, "linux", 1, 1, false},
	}
	for _, c := range cases {
		got, ok := pageZoomChord(c.e, c.goos, c.from)
		if got != c.want || ok != c.ok {
			t.Errorf("pageZoomChord(%q %v, %s, %v) = %v, %v; want %v, %v", c.e.Name, c.e.Modifiers, c.goos, c.from, got, ok, c.want, c.ok)
		}
	}
}
