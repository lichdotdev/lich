package main

import "testing"

func TestShortcutGroupsFollowTheWebsGroupsAndEndOnThePassthrough(t *testing.T) {
	groups := shortcutGroupsOf(defaultHotkeys(), "linux")
	var titles []string
	rows := 0
	for _, g := range groups {
		titles = append(titles, g.title)
		rows += len(g.rows)
	}
	want := []string{"Sessions", "View", "App", hotkeyPassthroughTitle}
	if len(titles) != len(want) {
		t.Fatalf("groups = %v, want %v", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("group %d = %q, want %q", i, titles[i], want[i])
		}
	}
	if total := len(hotkeyActions) + len(passthroughRows("linux")); rows != total {
		t.Errorf("rows = %d, want every action and every passed-through chord, %d", rows, total)
	}
}

func TestShortcutGroupsShowTheLiveBindings(t *testing.T) {
	bound := defaultHotkeys()
	bound[hotkeyNewSession] = combo{Mod: true, Alt: true, Key: "n"}
	bound[hotkeySettings] = comboUnassigned
	got := map[string]string{}
	for _, g := range shortcutGroupsOf(bound, "linux") {
		for _, r := range g.rows {
			got[r.label] = r.keys
		}
	}
	if got[hotkeyLabel(hotkeyNewSession)] != "Ctrl+Alt+N" {
		t.Errorf("rebound = %q", got[hotkeyLabel(hotkeyNewSession)])
	}
	if got[hotkeyLabel(hotkeySettings)] != hotkeyUnassigned {
		t.Errorf("unassigned = %q", got[hotkeyLabel(hotkeySettings)])
	}
}

func TestShortcutGroupsSpellTheChordsOfTheMac(t *testing.T) {
	for _, g := range shortcutGroupsOf(defaultHotkeys(), "darwin") {
		for _, r := range g.rows {
			if r.label == hotkeyLabel(hotkeyNewSession) && r.keys != "⌘⇧T" {
				t.Errorf("new session = %q", r.keys)
			}
		}
	}
}

func TestTheShortcutsChordIsCarriedOut(t *testing.T) {
	handler, ok := hotkeyHandlers[hotkeyShortcuts]
	if !ok {
		t.Fatal("the shortcuts action has no handler")
	}
	if handler.ready != nil {
		t.Error("the overlay opens anywhere, so the chord is never declined")
	}
	u := new(window)
	handler.run(u, C{})
	if !u.shortcuts.open {
		t.Fatal("the chord did not open the overlay")
	}
	handler.run(u, C{})
	if u.shortcuts.open {
		t.Error("the same chord did not close it")
	}
}
