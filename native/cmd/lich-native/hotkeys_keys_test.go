package main

import (
	"slices"
	"testing"

	"gioui.org/io/key"
)

func TestWebKeyNamesGiosKeysAsTheBrowserDoes(t *testing.T) {
	for name, want := range map[key.Name]string{
		key.NameReturn: "Enter", key.NameEnter: "Enter", key.NameDownArrow: "ArrowDown",
		key.NameEscape: "Escape", key.NameSpace: " ", key.NameDeleteBackward: "Backspace",
		key.NameF5: "F5", "T": "T", ",": ",", key.NameCtrl: "Control", key.NameCommand: "Meta",
	} {
		if got := webKey(name); got != want {
			t.Errorf("webKey(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestGioNamesIsTheInverseOfWebKeyForEveryDefault(t *testing.T) {
	for _, a := range hotkeyActions {
		if a.combo.Key == "" {
			continue
		}
		names := gioNames(a.combo.Key)
		if len(names) == 0 {
			t.Errorf("%s: no key name for %q", a.id, a.combo.Key)
		}
		for _, n := range names {
			if got := normalizeKey(webKey(n)); got != a.combo.Key {
				t.Errorf("%s: %q reads back as %q, want %q", a.id, n, got, a.combo.Key)
			}
		}
	}
}

func TestGioNamesSpellsPlusAsTheUnshiftedEqualsKeyToo(t *testing.T) {
	if got := gioNames("+"); !slices.Equal(got, []key.Name{"+", "="}) {
		t.Errorf("+ = %v", got)
	}
	if gioNames("Dead") != nil {
		t.Error("a key this window cannot name has filters")
	}
}

func TestComboFiltersAreExactSoCtrlShiftTIsNotCtrlT(t *testing.T) {
	filters := comboFilters(combo{Mod: true, Shift: true, Key: "t"}, "linux")
	var mods []key.Modifiers
	for _, f := range filters {
		if f.Optional != 0 || f.Focus != nil || f.Name != "T" {
			t.Errorf("filter %+v: optional modifiers, a focus tag or another name", f)
		}
		mods = append(mods, f.Required)
	}
	if want := []key.Modifiers{key.ModCtrl | key.ModShift, key.ModSuper | key.ModShift}; !slices.Equal(mods, want) {
		t.Errorf("modifiers = %v, want Ctrl and Super variants", mods)
	}
}

func TestComboFiltersTakeOnlyCommandForALetterOnMacOS(t *testing.T) {
	var mods []key.Modifiers
	for _, f := range comboFilters(combo{Mod: true, Key: "n"}, "darwin") {
		mods = append(mods, f.Required)
	}
	if !slices.Equal(mods, []key.Modifiers{key.ModCommand}) {
		t.Errorf("letter: %v", mods)
	}
	mods = nil
	for _, f := range comboFilters(combo{Mod: true, Key: "Tab"}, "darwin") {
		mods = append(mods, f.Required)
	}
	if !slices.Equal(mods, []key.Modifiers{key.ModCommand, key.ModCtrl}) {
		t.Errorf("Tab: %v", mods)
	}
}

func TestComboFiltersAreNoneForAnUnassignedAction(t *testing.T) {
	if got := comboFilters(comboUnassigned, "linux"); got != nil {
		t.Errorf("filters = %v", got)
	}
}

func TestKeyStateOfFoldsCommandAndSuperIntoMeta(t *testing.T) {
	got := keyStateOf(key.Event{Name: "T", Modifiers: key.ModCommand | key.ModShift})
	if want := (keyState{meta: true, shift: true, key: "T"}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !matchesCombo(keyStateOf(key.Event{Name: key.NameReturn, Modifiers: key.ModCtrl | key.ModShift}), combo{Mod: true, Shift: true, Key: "Enter"}) {
		t.Error("Ctrl+Shift+Return is not Ctrl+Shift+Enter")
	}
}
