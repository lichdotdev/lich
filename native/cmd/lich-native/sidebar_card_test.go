package main

import (
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func cardBodyHeight(sv sessionView, hovered bool) int {
	s := newSidebar(&ui.Root{})
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(300, 4000)}}
	more := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: image.Pt(16, 16)} }
	return s.cardBody(gtx, th, &sv, new(ui.Clickable), more, hovered).Size.Y
}

func TestCardIsOneLineInEveryState(t *testing.T) {
	idle := sessionView{ID: "s1", Label: "Docs", shown: "/home/me/repo", git: backend.DiffStats{Branch: "main"}}
	rest := cardBodyHeight(idle, false)
	busy := idle
	busy.status, busy.tool = statusBusy, sessionTool{Name: "Bash", Detail: "go test ./..."}
	waiting := idle
	waiting.status = statusWaiting
	pinned := idle
	pinned.Pinned = true
	git := idle
	git.pr = &backend.PullRequest{Number: 990}
	git.git = backend.DiffStats{Branch: "main", Files: 3, Added: 84, Deleted: 31}
	cases := map[string]struct {
		sv      sessionView
		hovered bool
	}{
		"tool": {busy, false}, "waiting": {waiting, false}, "pinned": {pinned, true}, "hovered": {idle, true}, "pr and diff": {git, false},
	}
	for name, c := range cases {
		if got := cardBodyHeight(c.sv, c.hovered); got != rest {
			t.Errorf("%s card is %dpx tall, at rest %dpx", name, got, rest)
		}
	}
}

func TestNewSessionHereOpensInTheCardsCheckoutAndFolder(t *testing.T) {
	s := &sidebar{launchProviders: []launchProvider{{id: "claude", name: "Claude Code"}}}
	sv := treeSession("a", "", "/wt/a")
	sv.Folder = "Apps"
	var act sidebarAction
	e := s.newSessionHere(&sv, &act)
	if len(e.Sub) == 0 {
		t.Fatal("the entry offers no provider")
	}
	e.Sub[0].OnClick()
	if want := (launchRequest{kind: "claude", dir: "/wt/a", folder: "Apps"}); act.launch != want {
		t.Fatalf("launch %+v, want %+v", act.launch, want)
	}
}
