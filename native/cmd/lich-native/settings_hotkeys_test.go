package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTheHotkeysSectionIsSearchableByEveryActionLabel(t *testing.T) {
	for _, a := range hotkeyActions {
		found := false
		for _, h := range searchSettings(a.label) {
			found = found || (h.section == sectionHotkeys && h.title == a.label)
		}
		if !found {
			t.Errorf("%q is not found in Hotkeys", a.label)
		}
	}
	hits := searchSettings("Command palette")
	if len(hits) == 0 || hits[0].path() != "Hotkeys › App" {
		t.Errorf("command palette: %+v; a row's path names its group", hits)
	}
}

func TestHotkeyRowsFollowTheGroupsInOrder(t *testing.T) {
	entries := sectionByID(sectionHotkeys).rows
	if len(entries) != len(hotkeyActions) {
		t.Fatalf("%d rows for %d actions", len(entries), len(hotkeyActions))
	}
	seen := map[string]bool{}
	last := ""
	for _, e := range entries {
		if e.group != last {
			if seen[e.group] {
				t.Fatalf("group %q comes back after another: the pane draws one heading per group", e.group)
			}
			seen[e.group], last = true, e.group
		}
	}
}

// What the web reads back is what native wrote, and the other way round.
func TestHotkeysShareTheWebsStoredShape(t *testing.T) {
	f, client := newSidebarBackend(t)
	s := newSettingsScreen(context.Background(), client, nil, func() {})
	if err := s.loadHotkeys(); err != nil || !equalBindings(s.hotkeys.current(), defaultHotkeys()) {
		t.Fatalf("nothing stored: %v", err)
	}
	s.setHotkey(hotkeyNewSession, combo{Mod: true, Shift: true, Key: "j"})
	s.setHotkey(hotkeyQuit, combo{Mod: true, Alt: true, Key: "q"})
	s.setHotkey(hotkeyDelegate, comboUnassigned)
	waitForSetting(t, f, hotkeysSettingKey+"@", mustStringify(t, s.hotkeys.current()))

	var raw map[string]map[string]any
	if err := json.Unmarshal([]byte(f.settings[hotkeysSettingKey+"@"]), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) != len(hotkeyActions) {
		t.Errorf("stored %d actions, want all %d as the web writes them", len(raw), len(hotkeyActions))
	}
	if got := raw["delegate"]; got["key"] != "" || got["mod"] != false {
		t.Errorf("delegate = %v, want the unassigned shape", got)
	}

	again := newSettingsScreen(context.Background(), client, nil, func() {})
	if err := again.loadHotkeys(); err != nil {
		t.Fatal(err)
	}
	if got := again.hotkeys.current()[hotkeyNewSession]; got.Key != "j" {
		t.Errorf("reopened with newSession = %+v", got)
	}
	again.resetHotkey(hotkeyNewSession)
	if got := again.hotkeys.current()[hotkeyNewSession]; got != defaultHotkeys()[hotkeyNewSession] {
		t.Errorf("after reset: %+v", got)
	}
}

func TestACorruptStoredValueReadsAsTheDefaultsAndNeverFailsTheLaunch(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.settings[hotkeysSettingKey+"@"] = `{"newSession":`
	s := newSettingsScreen(context.Background(), client, nil, func() {})
	if err := s.loadHotkeys(); err != nil || !equalBindings(s.hotkeys.current(), defaultHotkeys()) {
		t.Fatalf("err %v, bindings %v", err, s.hotkeys.current())
	}
}

func equalBindings(a, b hotkeyBindings) bool {
	if len(a) != len(b) {
		return false
	}
	for id, c := range a {
		if b[id] != c {
			return false
		}
	}
	return true
}

func mustStringify(t *testing.T, b hotkeyBindings) string {
	t.Helper()
	raw, err := stringifyHotkeys(b)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPassthroughRowsSpellTheImageChordPerPlatform(t *testing.T) {
	for goos, want := range map[string]string{"linux": "Ctrl+V", "windows": "Alt+V", "darwin": "⌘V or Ctrl+V"} {
		if got := passthroughRows(goos)[0].keys; got != want {
			t.Errorf("%s: %q, want %q", goos, got, want)
		}
	}
}
