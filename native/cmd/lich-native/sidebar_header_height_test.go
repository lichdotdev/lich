package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// A group header is a row tall, so the rail's nodes keep their height across
// the morph and its hover matches a row's.
func TestAFolderHeaderIsARowTall(t *testing.T) {
	s := newSidebar(&ui.Root{})
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	var r input.Router
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Source: r.Source(), Now: time.Unix(100, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(240, 4000)}}
	b := blockView{key: folderKey("Later"), kind: blockFolder, folder: "Later", title: "Later", sessions: []sessionView{treeSession("a", "", "/a")}}
	b.sessions[0].projectID = "p"
	st := s.blockState(b)
	var act sidebarAction
	d := s.groupHeader(gtx, th, b, st, blockFrame{act: &act})
	if want := gtx.Dp(railRow); d.Size.Y != want {
		t.Errorf("folder header is %dpx tall, want a row's %dpx", d.Size.Y, want)
	}
}
