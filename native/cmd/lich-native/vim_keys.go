package main

// Gio's keys in Neovim's key notation (:help key-notation), as nvim_input
// takes them. Text typed arrives as an EditEvent and goes as it is; a key
// event goes only for what text cannot carry: a named key, or a letter
// held with Ctrl, Alt or Command.

import (
	"strings"

	"gioui.org/io/key"
)

var vimKeyNames = map[key.Name]string{
	key.NameEscape: "Esc", key.NameReturn: "CR", key.NameEnter: "CR", key.NameTab: "Tab",
	key.NameDeleteBackward: "BS", key.NameDeleteForward: "Del", key.NameInsert: "Insert",
	key.NameUpArrow: "Up", key.NameDownArrow: "Down", key.NameLeftArrow: "Left", key.NameRightArrow: "Right",
	key.NameHome: "Home", key.NameEnd: "End", key.NamePageUp: "PageUp", key.NamePageDown: "PageDown",
	key.NameF1: "F1", key.NameF2: "F2", key.NameF3: "F3", key.NameF4: "F4", key.NameF5: "F5", key.NameF6: "F6",
	key.NameF7: "F7", key.NameF8: "F8", key.NameF9: "F9", key.NameF10: "F10", key.NameF11: "F11", key.NameF12: "F12",
}

// vimKeyOf is ev as nvim_input spells it, false for an event the text it
// types already carries, or none at all.
func vimKeyOf(ev key.Event) (string, bool) {
	if ev.State != key.Press {
		return "", false
	}
	mods := ev.Modifiers
	if mods.Contain(key.ModCtrl | key.ModAlt) {
		// AltGr arrives as Ctrl+Alt on some systems; what it types comes as text.
		return "", false
	}
	held := mods&(key.ModCtrl|key.ModAlt|key.ModCommand) != 0
	name, named := vimKeyNames[ev.Name]
	switch {
	case named:
	case ev.Name == key.NameSpace && held:
		name = "Space"
	case held && len(ev.Name) == 1:
		name = strings.ToLower(string(ev.Name))
		if name == "<" {
			name = "lt"
		}
	default:
		return "", false
	}
	var prefix strings.Builder
	for _, m := range []struct {
		mod key.Modifiers
		tag string
	}{{key.ModCtrl, "C-"}, {key.ModShift, "S-"}, {key.ModAlt, "M-"}, {key.ModCommand, "D-"}} {
		if mods.Contain(m.mod) {
			prefix.WriteString(m.tag)
		}
	}
	return "<" + prefix.String() + name + ">", true
}

// vimTextOf is typed text as nvim_input takes it: "<" starts a key name
// there, so it goes as <lt>.
func vimTextOf(text string) string {
	return strings.ReplaceAll(text, "<", "<lt>")
}
