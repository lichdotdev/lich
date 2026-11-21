package main

import (
	"slices"
	"strings"
	"unicode/utf8"

	"gioui.org/io/key"
)

// Gio's key names against the KeyboardEvent.key names a stored combo spells.
// Letters are the exception to the table: Gio names them upper case, and
// normalizeKey lowers them.
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
// and the KeyboardEvent.key name of each; comboFromEvent waits past them.
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
		if slices.Contains(k.gio, name) {
			return k.web
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

// chordModifiers are the modifier sets a combo's chord arrives with. The
// primary modifier is Ctrl or the Super key, and Command on macOS, where Ctrl
// also counts for a chord no terminal program reads as a control code: a
// letter's Ctrl chord belongs to the program in the terminal, and Ctrl+Tab
// stands in for Command+Tab, which is the system's.
func chordModifiers(c combo, goos string) []key.Modifiers {
	var base key.Modifiers
	if c.Shift {
		base |= key.ModShift
	}
	if c.Alt {
		base |= key.ModAlt
	}
	switch {
	case !c.Mod:
		return []key.Modifiers{base}
	case goos != "darwin":
		return []key.Modifiers{base | key.ModCtrl, base | key.ModSuper}
	case isLetter(c.Key):
		return []key.Modifiers{base | key.ModCommand}
	}
	return []key.Modifiers{base | key.ModCommand, base | key.ModCtrl}
}

func isLetter(k string) bool { return len(k) == 1 && 'a' <= k[0] && k[0] <= 'z' }

// comboFilters are the key filters that deliver c's chord and nothing else:
// the modifiers are required and none is optional, so Ctrl+Shift+T is not
// also Ctrl+T. No Focus, so the chord is the window's whoever holds the focus.
func comboFilters(c combo, goos string) []key.Filter {
	if c.Key == "" {
		return nil
	}
	var filters []key.Filter
	for _, name := range gioNames(c.Key) {
		for _, mods := range chordModifiers(c, goos) {
			filters = append(filters, key.Filter{Name: name, Required: mods})
		}
	}
	return filters
}

// spendsControlCode is a chord a terminal program reads as a control code,
// Ctrl and a letter, which shells, Claude Code, Codex and vim all bind.
func spendsControlCode(c combo) bool {
	return c.Mod && !c.Shift && !c.Alt && isLetter(c.Key)
}

// hotkeyFilters are comboFilters for a window hotkey, which leaves a focused
// terminal its control codes: there a Ctrl+letter hotkey answers to
// Ctrl+Shift+letter, a chord no terminal program binds, and anywhere else to
// both. Command reaches no terminal, so macOS has one chord everywhere.
func hotkeyFilters(c combo, goos string, inTerminal bool) []key.Filter {
	if goos == "darwin" || !spendsControlCode(c) {
		return comboFilters(c, goos)
	}
	shifted := c
	shifted.Shift = true
	if inTerminal {
		return comboFilters(shifted, goos)
	}
	return append(comboFilters(c, goos), comboFilters(shifted, goos)...)
}

// inTerminalCombo is the chord c answers to in a focused terminal, and false
// when that is c itself.
func inTerminalCombo(c combo, goos string) (combo, bool) {
	if goos == "darwin" || !spendsControlCode(c) {
		return c, false
	}
	c.Shift = true
	return c, true
}

// filtersTake reports whether one of filters delivers e.
func filtersTake(filters []key.Filter, e key.Event) bool {
	for _, f := range filters {
		if f.Name == e.Name && f.Required == e.Modifiers {
			return true
		}
	}
	return false
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
