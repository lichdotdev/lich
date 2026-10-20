package main

import (
	"reflect"
	"testing"
)

func TestMatchesComboAcceptsCtrlOrCmdForThePrimaryModifier(t *testing.T) {
	newSession := defaultHotkeys()[hotkeyNewSession] // Ctrl+Shift+T
	if !matchesCombo(keyState{ctrl: true, shift: true, key: "T"}, newSession) {
		t.Error("Ctrl+Shift+T does not match")
	}
	if !matchesCombo(keyState{meta: true, shift: true, key: "T"}, newSession) {
		t.Error("Cmd+Shift+T does not match")
	}
}

func TestMatchesComboFoldsEqualsIntoPlus(t *testing.T) {
	plus := combo{Mod: true, Key: "+"}
	if !matchesCombo(keyState{ctrl: true, key: "="}, plus) {
		t.Error("a combo recorded as + does not match the unshifted key")
	}
}

func TestMatchesComboIgnoresAutoRepeat(t *testing.T) {
	newSession := defaultHotkeys()[hotkeyNewSession]
	if matchesCombo(keyState{ctrl: true, shift: true, key: "T", repeat: true}, newSession) {
		t.Error("a held chord fired again")
	}
}

func TestMatchesComboRejectsADifferentModifierOrKey(t *testing.T) {
	newSession := defaultHotkeys()[hotkeyNewSession]
	for name, e := range map[string]keyState{
		"no shift":  {ctrl: true, key: "T"},
		"no mod":    {shift: true, key: "T"},
		"other key": {ctrl: true, shift: true, key: "N"},
	} {
		if matchesCombo(e, newSession) {
			t.Errorf("%s matched", name)
		}
	}
}

// The point of an unassigned action: nothing fires it, so the chord the user
// freed reaches the PTY the way it would if lich had never bound it.
func TestMatchesComboNeverMatchesAnUnassignedCombo(t *testing.T) {
	for _, e := range []keyState{{ctrl: true, shift: true, key: "T"}, {}, {ctrl: true, key: ""}} {
		if matchesCombo(e, comboUnassigned) {
			t.Errorf("%+v matched the unassigned combo", e)
		}
	}
}

func TestComboFromEventCapturesModifiersAndNormalizesTheKey(t *testing.T) {
	got, ok := comboFromEvent(keyState{ctrl: true, shift: true, key: "T"})
	if want := (combo{Mod: true, Shift: true, Key: "t"}); !ok || got != want {
		t.Errorf("got %+v, %v", got, ok)
	}
}

func TestComboFromEventWaitsPastABareModifier(t *testing.T) {
	if _, ok := comboFromEvent(keyState{ctrl: true, key: "Control"}); ok {
		t.Error("a bare modifier was captured")
	}
}

func TestComboFromEventNeedsAPrimaryModifierOrAlt(t *testing.T) {
	for _, e := range []keyState{{key: "t"}, {shift: true, key: "T"}} {
		if _, ok := comboFromEvent(e); ok {
			t.Errorf("%+v was captured: a bare key would fire while typing", e)
		}
	}
}

func TestComboFromEventAcceptsAltOnlyCombos(t *testing.T) {
	got, ok := comboFromEvent(keyState{alt: true, key: "n"})
	if want := (combo{Alt: true, Key: "n"}); !ok || got != want {
		t.Errorf("got %+v, %v", got, ok)
	}
}

func TestFormatCombo(t *testing.T) {
	c := combo{Mod: true, Shift: true, Key: "t"}
	if got := formatCombo(c, false); got != "Ctrl+Shift+T" {
		t.Errorf("off macOS: %q", got)
	}
	if got := formatCombo(c, true); got != "⌘⇧T" {
		t.Errorf("on macOS: %q", got)
	}
	for _, isMac := range []bool{false, true} {
		if got := formatCombo(comboUnassigned, isMac); got != "Unassigned" {
			t.Errorf("unassigned, mac=%v: %q", isMac, got)
		}
	}
	if got := formatCombo(combo{Mod: true, Shift: true, Key: ""}, false); got != "Unassigned" {
		t.Errorf("modifiers with no key: %q", got)
	}
}

func TestFormatComboNamesArrowsAndSpace(t *testing.T) {
	if got := formatCombo(combo{Mod: true, Shift: true, Key: "ArrowDown"}, false); got != "Ctrl+Shift+Down" {
		t.Errorf("arrow: %q", got)
	}
	if got := formatCombo(combo{Mod: true, Key: " "}, false); got != "Ctrl+Space" {
		t.Errorf("space: %q", got)
	}
	if got := formatCombo(combo{Mod: true, Alt: true, Key: "Enter"}, false); got != "Ctrl+Alt+Enter" {
		t.Errorf("named key: %q", got)
	}
}

func TestMergeHotkeysLayersAValidOverrideOverTheDefaults(t *testing.T) {
	override := map[string]any{"newSession": map[string]any{"mod": true, "shift": false, "alt": true, "key": "n"}}
	merged := mergeHotkeys(override)
	if want := (combo{Mod: true, Alt: true, Key: "n"}); merged[hotkeyNewSession] != want {
		t.Errorf("newSession = %+v", merged[hotkeyNewSession])
	}
	if merged[hotkeyCommandPalette] != defaultHotkeys()[hotkeyCommandPalette] {
		t.Errorf("commandPalette = %+v", merged[hotkeyCommandPalette])
	}
}

// What an install predating a new action has on disk.
func TestMergeHotkeysDefaultsAnActionAbsentFromStoredSettings(t *testing.T) {
	merged := mergeHotkeys(map[string]any{"newSession": map[string]any{"mod": true, "shift": false, "alt": true, "key": "n"}})
	if merged[hotkeyShortcuts] != defaultHotkeys()[hotkeyShortcuts] {
		t.Errorf("shortcuts = %+v", merged[hotkeyShortcuts])
	}
}

// Well formed enough to survive validation, but matchesCombo compares against
// a normalized event key: kept as "T" the shortcut would be dead.
func TestMergeHotkeysNormalizesAStoredKeySoAnUppercaseOneStillFires(t *testing.T) {
	merged := mergeHotkeys(map[string]any{"newSession": map[string]any{"mod": true, "shift": true, "alt": false, "key": "T"}})
	if merged[hotkeyNewSession].Key != "t" {
		t.Fatalf("key = %q", merged[hotkeyNewSession].Key)
	}
	if !matchesCombo(keyState{ctrl: true, shift: true, key: "T"}, merged[hotkeyNewSession]) {
		t.Error("the stored shortcut is dead")
	}
}

func TestMergeHotkeysIgnoresIDsThatAreNoLongerActions(t *testing.T) {
	merged := mergeHotkeys(map[string]any{"zoomIn": map[string]any{"mod": true, "shift": false, "alt": false, "key": "+"}})
	if !reflect.DeepEqual(merged, defaultHotkeys()) {
		t.Errorf("merged = %v", merged)
	}
}

func TestMergeHotkeysKeepsAnUnassignedActionUnassigned(t *testing.T) {
	merged := mergeHotkeys(map[string]any{"newSession": map[string]any{"mod": false, "shift": false, "alt": false, "key": ""}})
	if merged[hotkeyNewSession] != comboUnassigned {
		t.Errorf("newSession = %+v", merged[hotkeyNewSession])
	}
}

// What a hand-edited store can hold: nothing to press, but flags set. Two
// shapes for one nothing would read as a conflict and as a live binding.
func TestMergeHotkeysFoldsModifiersWithNoKeyIntoTheOneUnassignedShape(t *testing.T) {
	merged := mergeHotkeys(map[string]any{"newSession": map[string]any{"mod": true, "shift": true, "alt": true, "key": ""}})
	if merged[hotkeyNewSession] != comboUnassigned {
		t.Errorf("newSession = %+v", merged[hotkeyNewSession])
	}
}

func TestMergeHotkeysDropsMalformedEntriesAndNonObjects(t *testing.T) {
	for name, stored := range map[string]any{
		"malformed": map[string]any{"newSession": map[string]any{"mod": 1.0, "key": ""}},
		"null":      nil,
		"string":    "nope",
	} {
		if !reflect.DeepEqual(mergeHotkeys(stored), defaultHotkeys()) {
			t.Errorf("%s was not dropped", name)
		}
	}
}

func TestHotkeyConflictsReportsNothingWhenEveryActionHoldsItsOwnCombo(t *testing.T) {
	if got := hotkeyConflicts(defaultHotkeys()); len(got) != 0 {
		t.Errorf("conflicts = %v", got)
	}
}

func TestHotkeyConflictsNamesTheOtherActionOnBothSides(t *testing.T) {
	clashing := defaultHotkeys()
	clashing[hotkeyNewSession] = clashing[hotkeyCommandPalette]
	got := hotkeyConflicts(clashing)
	if !reflect.DeepEqual(got[hotkeyNewSession], []hotkeyID{hotkeyCommandPalette}) {
		t.Errorf("newSession: %v", got[hotkeyNewSession])
	}
	if !reflect.DeepEqual(got[hotkeyCommandPalette], []hotkeyID{hotkeyNewSession}) {
		t.Errorf("commandPalette: %v", got[hotkeyCommandPalette])
	}
	if _, ok := got[hotkeyNextSession]; ok {
		t.Error("nextSession reported")
	}
}

func TestHotkeyConflictsNamesBothOthersWhenThreeShareACombo(t *testing.T) {
	shared := combo{Mod: true, Shift: true, Key: "j"}
	b := defaultHotkeys()
	b[hotkeyNewSession], b[hotkeyNextSession], b[hotkeyPrevSession] = shared, shared, shared
	if got := hotkeyConflicts(b)[hotkeyNextSession]; !reflect.DeepEqual(got, []hotkeyID{hotkeyNewSession, hotkeyPrevSession}) {
		t.Errorf("nextSession: %v", got)
	}
}

// Nothing is not a chord two actions can both hold.
func TestHotkeyConflictsNeverReportsAnUnassignedAction(t *testing.T) {
	b := defaultHotkeys()
	b[hotkeyNewSession], b[hotkeyNextSession] = comboUnassigned, comboUnassigned
	if got := hotkeyConflicts(b); len(got) != 0 {
		t.Errorf("conflicts = %v", got)
	}
}

func TestHotkeyConflictsTreatsCombosDifferingInAModifierAsDistinct(t *testing.T) {
	b := defaultHotkeys()
	withAlt := b[hotkeyCommandPalette]
	withAlt.Alt = true
	b[hotkeyNewSession] = withAlt
	if got := hotkeyConflicts(b); len(got) != 0 {
		t.Errorf("conflicts = %v", got)
	}
}

func TestSameComboSeparatesUnassignedFromARealBinding(t *testing.T) {
	d := defaultHotkeys()
	if d[hotkeyNewSession] != d[hotkeyNewSession] || d[hotkeyNewSession] == d[hotkeyCommandPalette] {
		t.Error("compares every field")
	}
	if comboUnassigned != comboUnassigned || comboUnassigned == d[hotkeyNewSession] {
		t.Error("unassigned is a binding")
	}
}

func TestParseHotkeysAnswersTheDefaultsForAValueNobodyHasSet(t *testing.T) {
	if !reflect.DeepEqual(parseHotkeys(""), defaultHotkeys()) {
		t.Error("not the defaults")
	}
}

func TestParseHotkeysRoundTripsWhatTheEditorStores(t *testing.T) {
	mine := combo{Mod: true, Shift: true, Key: "j"}
	b := defaultHotkeys()
	b[hotkeyNewSession] = mine
	raw, err := stringifyHotkeys(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseHotkeys(raw)[hotkeyNewSession]; got != mine {
		t.Errorf("newSession = %+v", got)
	}
}

func TestParseHotkeysRoundTripsAnUnassignedActionAndTakesItsDefaultBackOnReset(t *testing.T) {
	b := defaultHotkeys()
	b[hotkeyNewSession] = comboUnassigned
	raw, _ := stringifyHotkeys(b)
	cleared := parseHotkeys(raw)
	if cleared[hotkeyNewSession] != comboUnassigned {
		t.Fatalf("cleared = %+v", cleared[hotkeyNewSession])
	}
	if matchesCombo(keyState{ctrl: true, shift: true, key: "T"}, cleared[hotkeyNewSession]) {
		t.Error("an unassigned action fired")
	}
	cleared[hotkeyNewSession] = defaultHotkeys()[hotkeyNewSession]
	again, _ := stringifyHotkeys(cleared)
	if !reflect.DeepEqual(parseHotkeys(again), defaultHotkeys()) {
		t.Error("reset did not restore the defaults")
	}
}

// A stored setting must never be able to break a launch: the value is a
// string somebody can hand-edit, and half of one is what an interrupted write
// leaves.
func TestParseHotkeysFallsBackToTheDefaultsForAValueThatIsNotJSON(t *testing.T) {
	for _, raw := range []string{`{"newSession":`, `"ctrl+shift+t"`} {
		if !reflect.DeepEqual(parseHotkeys(raw), defaultHotkeys()) {
			t.Errorf("%q did not fall back", raw)
		}
	}
}

func TestParseHotkeysKeepsAValidOverrideAndIgnoresWhatIsNotAnAction(t *testing.T) {
	got := parseHotkeys(`{"newSession":{"mod":true,"shift":true,"alt":false,"key":"J"},"zoomIn":{"mod":true,"shift":false,"alt":false,"key":"+"}}`)
	if got[hotkeyNewSession].Key != "j" {
		t.Errorf("newSession.key = %q", got[hotkeyNewSession].Key)
	}
	if _, ok := got["zoomIn"]; ok {
		t.Error("zoomIn survived")
	}
}

func TestHotkeyLabel(t *testing.T) {
	if got := hotkeyLabel(hotkeyCommandPalette); got != "Command palette" {
		t.Errorf("label = %q", got)
	}
	// Nothing in the app can ask for an id that is not an action, but the
	// label is what a settings row renders: an empty one would be a blank line.
	if got := hotkeyLabel("zoomIn"); got != "zoomIn" {
		t.Errorf("unknown = %q", got)
	}
}

func TestTerminalCostNamesWhatARebindSpendsAndNothingForAFreeChord(t *testing.T) {
	cost := terminalCost(combo{Mod: true, Key: "r"})
	if want := "Ctrl+R is the shell's history search; sessions will no longer see it."; cost != want {
		t.Errorf("Ctrl+R: %q", cost)
	}
	if got := terminalCost(combo{Mod: true, Key: "Backspace"}); got != "Ctrl+Backspace is the erase word lich sends the agent; sessions will no longer see it." {
		t.Errorf("Ctrl+Backspace: %q", got)
	}
	for name, c := range map[string]combo{
		"with shift": {Mod: true, Shift: true, Key: "r"},
		"with alt":   {Mod: true, Alt: true, Key: "r"},
		"no mod":     {Key: "r"},
		"free key":   {Mod: true, Key: "j"},
		"unassigned": comboUnassigned,
	} {
		if got := terminalCost(c); got != "" {
			t.Errorf("%s: %q", name, got)
		}
	}
}

func TestOnlyCommandPaletteDefaultsToAChordTheTerminalSpends(t *testing.T) {
	// Ctrl+K is the one plain-Ctrl+letter default; the rest are
	// Ctrl+Shift+letter, which no TUI sees, and Ctrl+, and Ctrl+/.
	costly := map[hotkeyID]bool{}
	for _, a := range hotkeyActions {
		if terminalCost(a.combo) != "" {
			costly[a.id] = true
		}
	}
	if !reflect.DeepEqual(costly, map[hotkeyID]bool{hotkeyCommandPalette: true}) {
		t.Errorf("defaults that cost the terminal: %v", costly)
	}
}
