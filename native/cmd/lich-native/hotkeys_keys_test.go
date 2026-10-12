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
	filters := comboFilters(defaultHotkeys()[hotkeyNewSession])
	want := key.ModCtrl | key.ModShift
	var ctrl *key.Filter
	for i, f := range filters {
		if f.Required == want {
			ctrl = &filters[i]
		}
		if f.Optional != 0 || f.Focus != nil || f.Name != "T" {
			t.Errorf("filter %+v: optional modifiers, a focus tag or another name", f)
		}
	}
	if ctrl == nil {
		t.Fatal("no Ctrl+Shift filter")
	}
	if len(filters) != 3 {
		t.Errorf("filters = %d, want Ctrl, Command and Super variants", len(filters))
	}
}

func TestComboFiltersAreNoneForAnUnassignedAction(t *testing.T) {
	if got := comboFilters(comboUnassigned); got != nil {
		t.Errorf("filters = %v", got)
	}
}

func TestKeyStateOfFoldsCommandAndSuperIntoMeta(t *testing.T) {
	got := keyStateOf(key.Event{Name: "T", Modifiers: key.ModCommand | key.ModShift})
	if want := (keyState{meta: true, shift: true, key: "T"}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !matchesCombo(keyStateOf(key.Event{Name: key.NameReturn, Modifiers: key.ModCtrl | key.ModShift}), defaultHotkeys()[hotkeyFocusTerminal]) {
		t.Error("Ctrl+Shift+Return is not focusTerminal")
	}
}
