package main

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
)

// domKey is a key as a page's KeyboardEvent reads it: key, code, and the
// Windows virtual key code Chrome derives the rest from.
type domKey struct {
	key, code string
	keyCode   int
}

// browserNamedKeys are the keys that type no text, by Gio's name.
var browserNamedKeys = map[key.Name]domKey{
	key.NameReturn:         {"Enter", "Enter", 13},
	key.NameEnter:          {"Enter", "NumpadEnter", 13},
	key.NameTab:            {"Tab", "Tab", 9},
	key.NameDeleteBackward: {"Backspace", "Backspace", 8},
	key.NameDeleteForward:  {"Delete", "Delete", 46},
	key.NameEscape:         {"Escape", "Escape", 27},
	key.NameLeftArrow:      {"ArrowLeft", "ArrowLeft", 37},
	key.NameUpArrow:        {"ArrowUp", "ArrowUp", 38},
	key.NameRightArrow:     {"ArrowRight", "ArrowRight", 39},
	key.NameDownArrow:      {"ArrowDown", "ArrowDown", 40},
	key.NameHome:           {"Home", "Home", 36},
	key.NameEnd:            {"End", "End", 35},
	key.NamePageUp:         {"PageUp", "PageUp", 33},
	key.NamePageDown:       {"PageDown", "PageDown", 34},
	key.NameSpace:          {" ", "Space", 32},
}

// browserKey is the page's key for a Gio key event, false for one whose text
// reaches the page as committed text instead.
func browserKey(id string, e key.Event) (api.BrowserKeyOptions, bool) {
	k, ok := browserNamedKeys[e.Name]
	if !ok {
		k, ok = characterDomKey(e.Name, e.Modifiers)
	}
	if !ok {
		return api.BrowserKeyOptions{}, false
	}
	kind := "down"
	if e.State == key.Release {
		kind = "up"
	}
	return api.BrowserKeyOptions{ID: id, Type: kind, Key: k.key, Code: k.code, KeyCode: k.keyCode, Modifiers: cdpModifiers(e.Modifiers)}, true
}

// characterDomKey is a letter or digit typed with Ctrl, Alt or Command,
// which carries no EditEvent: the shortcuts a page handles itself.
func characterDomKey(name key.Name, mods key.Modifiers) (domKey, bool) {
	if mods&(key.ModCtrl|key.ModAlt|key.ModCommand) == 0 || len(name) != 1 {
		return domKey{}, false
	}
	c := name[0]
	switch {
	case 'A' <= c && c <= 'Z':
		letter := strings.ToLower(string(c))
		if mods.Contain(key.ModShift) {
			letter = string(c)
		}
		return domKey{letter, "Key" + string(c), int(c)}, true
	case '0' <= c && c <= '9':
		return domKey{string(c), "Digit" + string(c), int(c)}, true
	}
	return domKey{}, false
}

// cdpModifiers is CDP's modifier bit set: Alt 1, Ctrl 2, Meta 4, Shift 8.
func cdpModifiers(m key.Modifiers) int {
	out := 0
	for _, bit := range []struct {
		gio key.Modifiers
		cdp int
	}{{key.ModAlt, 1}, {key.ModCtrl, 2}, {key.ModSuper, 4}, {key.ModShift, 8}} {
		if m.Contain(bit.gio) {
			out |= bit.cdp
		}
	}
	if key.ModShortcut == key.ModCommand && m.Contain(key.ModCommand) {
		out |= 4
	}
	return out
}

// browserButton is CDP's name for a Gio pointer button, "" for none.
func browserButton(b pointer.Buttons) string {
	switch {
	case b.Contain(pointer.ButtonPrimary):
		return "left"
	case b.Contain(pointer.ButtonSecondary):
		return "right"
	case b.Contain(pointer.ButtonTertiary):
		return "middle"
	}
	return ""
}
