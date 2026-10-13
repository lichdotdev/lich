package main

import (
	"context"
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

var twoProviders = []launchProvider{{id: "claude", name: "Claude Code"}, {id: "codex", name: "Codex"}}

func pickedLaunch(t *testing.T, entries []ui.MenuEntry, act *sidebarAction, path ...string) launchRequest {
	t.Helper()
	for _, label := range path[:len(path)-1] {
		entries = entry(t, entries, label).Sub
	}
	entry(t, entries, path[len(path)-1]).OnClick()
	return act.launch
}

func TestBlockLaunchOpensInTheBlocksCheckoutOrAsksAFoldersWhere(t *testing.T) {
	var act sidebarAction
	if blockLaunch(blockView{kind: blockPinned}, twoProviders, &act) != nil {
		t.Error("the pinned block offers a +")
	}
	checkout := blockLaunch(blockView{kind: blockCheckout, path: "/wt/a"}, twoProviders, &act)
	if want := []string{"Claude Code", "Codex", "", "New Terminal"}; !slices.Equal(labels(checkout), want) {
		t.Fatalf("checkout + %v, want %v", labels(checkout), want)
	}
	if got := pickedLaunch(t, checkout, &act, "New Terminal"); got != (launchRequest{kind: "shell", dir: "/wt/a"}) {
		t.Errorf("New Terminal launched %+v", got)
	}

	one := blockView{kind: blockFolder, folder: "Apps", title: "Apps", checkouts: []launchCheckout{{path: "/wt/a", label: "a"}}}
	menu := blockLaunch(one, twoProviders, &act)
	if want := []string{"New session in Apps, on a", "Claude Code", "Codex", "", "New Terminal"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("one-checkout folder + %v, want %v", labels(menu), want)
	}
	if got := pickedLaunch(t, menu, &act, "Codex"); got != (launchRequest{kind: "codex", dir: "/wt/a", folder: "Apps"}) {
		t.Errorf("Codex launched %+v", got)
	}

	two := one
	two.checkouts = append(two.checkouts, launchCheckout{path: "", label: "repo"})
	menu = blockLaunch(two, twoProviders, &act)
	if want := []string{"New session in Apps", "a", "repo"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("two-checkout folder + %v, want %v", labels(menu), want)
	}
	if got := pickedLaunch(t, menu, &act, "repo", "Claude Code"); got != (launchRequest{kind: "claude", folder: "Apps"}) {
		t.Errorf("repo › Claude Code launched %+v", got)
	}
}

func TestBlockOptionsByKindAndFilter(t *testing.T) {
	var act sidebarAction
	renamed, asked := false, []string(nil)
	hooks := blockMenuHooks{rename: func() { renamed = true }, newFolder: func(ids []string) { asked = ids }}
	folder := blockView{kind: blockFolder, folder: "Apps", color: "teal", sessions: views("f1", "f2")}
	checkout := blockView{kind: blockCheckout, sessions: views("r1", "r2")}

	if blockOptions(blockView{kind: blockPinned}, nil, false, &act, hooks) != nil {
		t.Error("the pinned block has a … menu")
	}
	menu := blockOptions(folder, []string{"Apps"}, false, &act, hooks)
	if want := []string{"Rename folder", "Ungroup", "Color"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("folder … %v, want %v", labels(menu), want)
	}
	entry(t, menu, "Rename folder").OnClick()
	entry(t, menu, "Ungroup").OnClick()
	entry(t, entry(t, menu, "Color").Sub, "Red").OnClick()
	if !renamed || act.folder != (folderRename{from: "Apps"}) || act.paint.color != "red" || !slices.Equal(act.paint.ids, []string{"f1", "f2"}) {
		t.Fatalf("after the clicks: renamed %v, act %+v", renamed, act.sidebarWrites)
	}
	if !entry(t, entry(t, menu, "Color").Sub, "Teal").Checked {
		t.Error("the folder's shared color is not checked")
	}
	if want := []string{"Rename folder", "Ungroup"}; !slices.Equal(labels(blockOptions(folder, nil, true, &act, hooks)), want) {
		t.Error("a filtered folder still recolors the cards it did not draw")
	}

	sub := entry(t, blockOptions(checkout, []string{"Apps", "Docs"}, false, &act, hooks), "Move group to folder").Sub
	if want := []string{"Apps", "Docs", "", "New folder…"}; !slices.Equal(labels(sub), want) {
		t.Fatalf("Move group to folder %v, want %v", labels(sub), want)
	}
	entry(t, sub, "Docs").OnClick()
	entry(t, sub, "New folder…").OnClick()
	if act.file.folder != "Docs" || !slices.Equal(act.file.ids, []string{"r1", "r2"}) || !slices.Equal(asked, []string{"r1", "r2"}) {
		t.Errorf("filed %+v, asked a name for %v", act.file, asked)
	}
	if blockOptions(checkout, []string{"Apps"}, true, &act, hooks) != nil {
		t.Error("a filtered checkout block files the cards it did not draw")
	}
}

func TestTakesCard(t *testing.T) {
	card := func(folder, path string) sessionView {
		return sessionView{Session: lichclient.Session{ID: "c", Folder: folder, Path: path}}
	}
	apps := blockView{kind: blockFolder, folder: "Apps"}
	root := blockView{kind: blockCheckout}
	wt := blockView{kind: blockCheckout, path: "/wt/a"}
	cases := []struct {
		name   string
		target blockView
		card   sessionView
		want   bool
	}{
		{"folder takes an unfiled card", apps, card("", ""), true},
		{"folder takes another folder's card", apps, card("Docs", "/wt/a"), true},
		{"folder refuses its own card", apps, card("Apps", ""), false},
		{"checkout takes its own card out of a folder", wt, card("Apps", "/wt/a"), true},
		{"root takes its own card out of a folder", root, card("Apps", ""), true},
		{"checkout refuses another checkout's card", root, card("Apps", "/wt/a"), false},
		{"checkout refuses an unfiled card", wt, card("", "/wt/a"), false},
		{"pinned takes nothing", blockView{kind: blockPinned}, card("Apps", ""), false},
	}
	for _, c := range cases {
		if got := takesCard(c.target, c.card); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

func TestArrayMove(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	if got := arrayMove(ids, "a", "c"); !slices.Equal(got, []string{"b", "c", "a", "d"}) {
		t.Errorf("down: %v", got)
	}
	if got := arrayMove(ids, "d", "b"); !slices.Equal(got, []string{"a", "d", "b", "c"}) {
		t.Errorf("up: %v", got)
	}
	if got := arrayMove(ids, "x", "b"); !slices.Equal(got, ids) {
		t.Errorf("unknown id: %v", got)
	}
	if !slices.Equal(ids, []string{"a", "b", "c", "d"}) {
		t.Error("arrayMove changed its input")
	}
}

func TestFolderNamingCommitAndMergeNote(t *testing.T) {
	if _, ok, _ := folderCommit("   ", nil); ok {
		t.Error("a blank name commits")
	}
	if name, ok, known := folderCommit(" Apps ", []string{"Apps"}); name != "Apps" || !ok || !known {
		t.Errorf("existing name: %q %v %v", name, ok, known)
	}
	if !mergesInto("Apps", "Docs", []string{"Apps", "Docs"}) || mergesInto("Apps", "Apps", []string{"Apps"}) || mergesInto("Apps", "New", []string{"Apps"}) {
		t.Error("mergesInto names the wrong renames")
	}
	if folderDescription(1) == folderDescription(2) {
		t.Error("one session and two read the same")
	}
}

func TestFilterToggleAndEscape(t *testing.T) {
	h := sidebarHeader{phases: map[sessionPhase]bool{}}
	h.toggle()
	h.query.SetText("api")
	h.phases[phaseWaiting] = true
	if f := h.filter(); f.query != "api" || !f.narrows() {
		t.Fatalf("open filter = %+v", f)
	}
	h.escape()
	if !h.open || h.query.Text() != "" || !h.phases[phaseWaiting] {
		t.Fatal("Escape with text did not just clear the query")
	}
	h.escape()
	if h.open || len(h.phases) != 0 || h.filter().narrows() {
		t.Fatal("Escape on an empty field did not close and clear the filter")
	}
	h.toggle()
	h.query.SetText("x")
	h.toggle()
	if h.filter().narrows() || h.query.Text() != "" {
		t.Error("closing the filter kept a query hiding cards")
	}
}

func TestTakeCardKeepsTheFramesWrites(t *testing.T) {
	act := sidebarAction{sidebarWrites: sidebarWrites{fold: "k", saveOpen: true}}
	act.takeCard(sidebarAction{activate: "s1", pin: "s1", pinTo: true, sidebarWrites: sidebarWrites{file: fileRequest{ids: []string{"s1"}, folder: "Apps"}}})
	act.takeCard(sidebarAction{})
	if act.activate != "s1" || act.pin != "s1" || !act.pinTo || act.fold != "k" || !act.saveOpen || act.file.folder != "Apps" {
		t.Errorf("act = %+v", act)
	}
}

func TestTakeCardKeepsAContinueIn(t *testing.T) {
	var act sidebarAction
	act.takeCard(sidebarAction{continueIn: continueRequest{id: "s1", slug: "work"}})
	act.takeCard(sidebarAction{})
	if act.continueIn != (continueRequest{id: "s1", slug: "work"}) {
		t.Errorf("continueIn = %+v, want the card's request", act.continueIn)
	}
}

func TestWriteSidebarStoresFoldsFilesAndLayout(t *testing.T) {
	stored := filed(ss("r1", "f1", "w1@/x/worktrees/p/feat"), "Apps", "f1")
	m, f := sidebarModel(t, stored)
	ctx := context.Background()
	const feat = "/x/worktrees/p/feat"
	w := sidebarWrites{fold: feat, file: fileRequest{ids: []string{"r1"}, folder: "Apps"}, saveWidth: true, saveOpen: true}
	if err := writeSidebar(ctx, m, w, sidebarLayout{widthRem: 20, open: false}); err != nil {
		t.Fatal(err)
	}
	if f.get(sidebarCollapsedKey+feat+"@p") != "true" {
		t.Error("the fold was not stored under the project")
	}
	if f.get(sidebarWidthKey+"@") != "20" || f.get(sidebarOpenKey+"@") != "false" {
		t.Errorf("layout stored as width %q open %q", f.get(sidebarWidthKey+"@"), f.get(sidebarOpenKey+"@"))
	}
	if want := []string{`store.SetSessionFolder("r1","Apps")`}; !slices.Equal(f.calls, want) {
		t.Errorf("calls %v, want %v", f.calls, want)
	}
	if v := m.sidebarBlocks(sidebarFilter{}); !slices.Equal(blockKeys(v), []string{folderKey("Apps"), feat}) || !blockFolded(v, feat) {
		t.Errorf("blocks after the writes: %v, feat folded %v", blockKeys(v), blockFolded(v, feat))
	}

	f.calls = nil
	if err := writeSidebar(ctx, m, sidebarWrites{paint: paintRequest{ids: []string{"r1", "f1"}, color: "red"}}, sidebarLayout{}); err != nil {
		t.Fatal(err)
	}
	if want := []string{`store.SetSessionColor("r1","red")`, `store.SetSessionColor("f1","red")`}; !slices.Equal(f.calls, want) {
		t.Errorf("paint calls %v, want %v", f.calls, want)
	}
	if v := m.sidebarBlocks(sidebarFilter{}); v.blocks[0].color != "red" {
		t.Errorf("folder color %q after the paint", v.blocks[0].color)
	}
}

func blockFolded(v sidebarView, key string) bool {
	for _, b := range v.blocks {
		if b.key == key {
			return b.collapsed
		}
	}
	return false
}
