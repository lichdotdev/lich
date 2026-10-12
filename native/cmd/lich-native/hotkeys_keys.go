package main

import (
	"strings"
	"unicode/utf8"

	"gioui.org/io/key"
)

// Gio's key names against the web's KeyboardEvent.key, which a stored combo
// spells. Letters are the exception to the table: Gio names them upper case,
// and normalizeKey lowers them.
var namedKeys = []struct {
	web string
	gio []key.Name
}{
	{"Enter", []key.Name{key.NameReturn, key.NameEnter}},
	{"ArrowUp", []key.Name{key.NameUpArrow}},
	{"ArrowDown", []key.Name{key.NameDownArrow}},
	{"ArrowLeft", []key.Name{key.NameLeftArrow}},
	{"ArrowRight", []key.Name{key.NameRightArrow}},
	{"Escape", []key.Name{key.NameEscape}},
	{"Tab", []key.Name{key.NameTab}},
	{" ", []key.Name{key.NameSpace}},
	{"Backspace", []key.Name{key.NameDeleteBackward}},
	{"Delete", []key.Name{key.NameDeleteForward}},
	{"Home", []key.Name{key.NameHome}},
	{"End", []key.Name{key.NameEnd}},
	{"PageUp", []key.Name{key.NamePageUp}},
	{"PageDown", []key.Name{key.NamePageDown}},
	{"F1", []key.Name{key.NameF1}},
	{"F2", []key.Name{key.NameF2}},
	{"F3", []key.Name{key.NameF3}},
	{"F4", []key.Name{key.NameF4}},
	{"F5", []key.Name{key.NameF5}},
	{"F6", []key.Name{key.NameF6}},
	{"F7", []key.Name{key.NameF7}},
	{"F8", []key.Name{key.NameF8}},
	{"F9", []key.Name{key.NameF9}},
	{"F10", []key.Name{key.NameF10}},
	{"F11", []key.Name{key.NameF11}},
	{"F12", []key.Name{key.NameF12}},
}

// modifierKeys are the key names Gio reports for a modifier pressed alone,
// and what the web calls each; comboFromEvent waits past them.
var modifierKeys = map[key.Name]string{
	key.NameCtrl: "Control", key.NameShift: "Shift", key.NameAlt: "Alt",
	key.NameSuper: "Meta", key.NameCommand: "Meta",
}

// webKey is the KeyboardEvent.key a Gio key name stands for.
func webKey(name key.Name) string {
	if web, ok := modifierKeys[name]; ok {
		return web
	}
	for _, k := range namedKeys {
		for _, g := range k.gio {
			if g == name {
				return k.web
			}
		}
	}
	return string(name)
}

// gioNames are the key names that spell a stored combo's key: none for a key
// this window cannot name. "+" is also the unshifted "=" key.
func gioNames(web string) []key.Name {
	if web == "+" {
		return []key.Name{"+", "="}
	}
	for _, k := range namedKeys {
		if k.web == web {
			return k.gio
		}
	}
	if utf8.RuneCountInString(web) == 1 {
		return []key.Name{key.Name(strings.ToUpper(web))}
	}
	return nil
}

// chordModifiers are the modifier sets a combo's chord arrives with: the
// primary modifier is Ctrl, Command or the Super key, as the web's
// ctrlKey || metaKey.
func chordModifiers(c combo) []key.Modifiers {
	var base key.Modifiers
	if c.Shift {
		base |= key.ModShift
	}
	if c.Alt {
		base |= key.ModAlt
	}
	if !c.Mod {
		return []key.Modifiers{base}
	}
	return []key.Modifiers{base | key.ModCtrl, base | key.ModCommand, base | key.ModSuper}
}

// comboFilters are the key filters that deliver c's chord and nothing else:
// the modifiers are required and none is optional, so Ctrl+Shift+T is not
// also Ctrl+T. No Focus, so the chord is the window's whoever holds the focus.
func comboFilters(c combo) []key.Filter {
	if c.Key == "" {
		return nil
	}
	var filters []key.Filter
	for _, name := range gioNames(c.Key) {
		for _, mods := range chordModifiers(c) {
			filters = append(filters, key.Filter{Name: name, Required: mods})
		}
	}
	return filters
}

// keyStateOf reads a Gio key event as the matcher's KeyState.
func keyStateOf(e key.Event) keyState {
	return keyState{
		ctrl:  e.Modifiers.Contain(key.ModCtrl),
		meta:  e.Modifiers&(key.ModCommand|key.ModSuper) != 0,
		shift: e.Modifiers.Contain(key.ModShift),
		alt:   e.Modifiers.Contain(key.ModAlt),
		key:   webKey(e.Name),
	}
}
