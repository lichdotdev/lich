package main

import (
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func cardBodyHeight(sv sessionView) int {
	s := newSidebar(&ui.Root{})
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(300, 4000)}}
	return s.cardBody(gtx, th, &sv, new(widget.Clickable)).Size.Y
}

func TestCardKeepsItsHeightWhileTheStatusRungShows(t *testing.T) {
	idle := sessionView{Session: backend.Session{ID: "s1", Label: "Docs"}, shown: "/home/me/repo", git: backend.DiffStats{Branch: "main"}}
	rest := cardBodyHeight(idle)
	busy := idle
	busy.status, busy.tool = statusBusy, sessionTool{Name: "Bash", Detail: "go test ./..."}
	waiting := idle
	waiting.status = statusWaiting
	for name, sv := range map[string]sessionView{"tool": busy, "waiting": waiting} {
		if got := cardBodyHeight(sv); got != rest {
			t.Errorf("%s card is %dpx tall, at rest %dpx", name, got, rest)
		}
	}
}
