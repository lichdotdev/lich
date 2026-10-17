package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

func labels(entries []ui.MenuEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Label)
	}
	return out
}

func entry(t *testing.T, entries []ui.MenuEntry, label string) ui.MenuEntry {
	t.Helper()
	i := slices.IndexFunc(entries, func(e ui.MenuEntry) bool { return e.Label == label })
	if i < 0 {
		t.Fatalf("no %q in %v", label, labels(entries))
	}
	return entries[i]
}

func TestCardMenu(t *testing.T) {
	var act sidebarAction
	renamed := false
	sv := &sessionView{Session: lichclient.Session{ID: "s1", Kind: "claude", Color: "teal"}, shown: "/repo"}
	menu := cardMenu(sv, nil, &act, func() { renamed = true }, func() {})

	if want := []string{"Rename", "Pin", "Color", "Move to folder", "Open in", "Pull request", "", "Close session"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("menu %v, want %v", labels(menu), want)
	}
	entry(t, menu, "Rename").OnClick()
	entry(t, menu, "Pin").OnClick()
	entry(t, entry(t, menu, "Color").Sub, "Theme").OnClick()
	entry(t, entry(t, menu, "Open in").Sub, "Terminal").OnClick()
	entry(t, menu, "Close session").OnClick()
	if !renamed || act.pin != "s1" || !act.pinTo || act.color != "s1" || act.colorTo != "" || act.terminalAt != "/repo" || act.close != "s1" {
		t.Fatalf("after the clicks: renamed %v, act %+v", renamed, act)
	}
	if !entry(t, entry(t, menu, "Color").Sub, "Teal").Checked {
		t.Error("the card's own color is not checked")
	}
	if !entry(t, menu, "Pull request").Disabled {
		t.Error("Pull request enabled on a branch without one")
	}

	sv.Kind, sv.Pinned = "shell", true
	menu = cardMenu(sv, nil, &act, func() {}, func() {})
	if slices.Contains(labels(menu), "Close session") || entry(t, menu, "Unpin").Label != "Unpin" {
		t.Errorf("pinned menu %v: want Unpin and no Close session", labels(menu))
	}
	if slices.Contains(labels(entry(t, menu, "Open in").Sub), "Terminal") {
		t.Error("a shell session offers a terminal at its own directory")
	}
}

func TestCardMenuFilesIntoAnotherFolderOrOutOfItsOwn(t *testing.T) {
	var act sidebarAction
	asked := false
	sv := &sessionView{Session: lichclient.Session{ID: "s1", Folder: "Apps"}}
	sub := entry(t, cardMenu(sv, []string{"Docs"}, &act, func() {}, func() { asked = true }), "Move to folder").Sub
	if want := []string{"Docs", "Take out of Apps", "", "New folder…"}; !slices.Equal(labels(sub), want) {
		t.Fatalf("submenu %v, want %v", labels(sub), want)
	}
	entry(t, sub, "Docs").OnClick()
	if !slices.Equal(act.file.ids, []string{"s1"}) || act.file.folder != "Docs" {
		t.Errorf("Docs filed %+v", act.file)
	}
	entry(t, sub, "Take out of Apps").OnClick()
	if !slices.Equal(act.file.ids, []string{"s1"}) || act.file.folder != "" {
		t.Errorf("Take out filed %+v", act.file)
	}
	entry(t, sub, "New folder…").OnClick()
	if !asked {
		t.Error("New folder… did not ask for a name")
	}

	sv.Folder = ""
	sub = entry(t, cardMenu(sv, nil, &act, func() {}, func() {}), "Move to folder").Sub
	if want := []string{"New folder…"}; !slices.Equal(labels(sub), want) {
		t.Errorf("unfiled card with no folders: %v, want %v", labels(sub), want)
	}
}

func TestCardMenuForkEntry(t *testing.T) {
	var act sidebarAction
	sv := &sessionView{Session: lichclient.Session{ID: "s1", Kind: "claude", ProviderSessionID: "conv-1"}}
	fork := entry(t, cardMenu(sv, nil, &act, func() {}, func() {}), "Fork")
	if fork.Kind != ui.MenuSubTrigger || !slices.Equal(labels(fork.Sub), []string{"Here", "Worktree…"}) {
		t.Fatalf("fork entry %+v, want a submenu of Here and Worktree…", labels(fork.Sub))
	}
	entry(t, fork.Sub, "Here").OnClick()
	entry(t, fork.Sub, "Worktree…").OnClick()
	if act.forkHere != "s1" || act.forkWorktree != "s1" {
		t.Fatalf("act %+v, want both forks asked for s1", act)
	}

	sv.Kind = "crush"
	dead := entry(t, cardMenu(sv, nil, &act, func() {}, func() {}), "Fork")
	if !dead.Disabled || dead.Sub != nil || dead.Shortcut == "" {
		t.Errorf("crush fork entry %+v, want it disabled, saying why, with no submenu", dead)
	}

	for _, s := range []lichclient.Session{
		{ID: "s1", Kind: "claude"},                             // no conversation to branch yet
		{ID: "s1", Kind: "shell", ProviderSessionID: "conv-1"}, // nothing a shell could fork
	} {
		sv.Session = s
		if slices.Contains(labels(cardMenu(sv, nil, &act, func() {}, func() {})), "Fork") {
			t.Errorf("%s session with conversation %q offers Fork", s.Kind, s.ProviderSessionID)
		}
	}
}
