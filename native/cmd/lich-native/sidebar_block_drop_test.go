package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// A card let go over another folder's rows, not its header, is filed there:
// the whole block takes the drop.
func TestACardDroppedOnAFoldersRowsIsFiledThere(t *testing.T) {
	s := newSidebar(&ui.Root{})
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	folder := func(name string, ids ...string) blockView {
		b := blockView{key: folderKey(name), kind: blockFolder, folder: name, title: name}
		for _, id := range ids {
			c := treeSession(id, "", "/"+id)
			c.projectID, c.Folder = "p", name
			b.sessions = append(b.sessions, c)
		}
		return b
	}
	view := sidebarView{project: "p", blocks: []blockView{folder("Doing", "a"), folder("Done", "b", "c")}, folders: []string{"Doing", "Done"}}
	var r input.Router
	var ops op.Ops
	now := time.Unix(100, 0)
	frame := func() sidebarAction {
		var act sidebarAction
		ops.Reset()
		now = now.Add(time.Second)
		gtx := layout.Context{Ops: &ops, Source: r.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(240, 4000)}}
		f := blockFrame{view: view, settled: true, act: &act}
		children := make([]layout.FlexChild, len(view.blocks))
		for i, b := range view.blocks {
			children[i] = layout.Rigid(func(gtx C) D { return s.block(gtx, th, b, f) })
		}
		layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		r.Frame(&ops)
		return act
	}
	// Doing: header, a. Done: header, b, c. Each a row tall, a Dp a pixel.
	row := float32(railRow)
	cardA := f32.Pt(100, row*1.5)
	rowOfDone := f32.Pt(100, row*4.5)
	move := func(kind pointer.Kind, at f32.Point) {
		r.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at})
	}
	frame()
	frame()
	move(pointer.Move, cardA)
	frame()
	move(pointer.Press, cardA)
	frame()
	for _, y := range []float32{0.1, 0.5, 0.9} {
		move(pointer.Move, cardA.Add(f32.Pt(0, (rowOfDone.Y-cardA.Y)*y)))
		frame()
	}
	move(pointer.Move, rowOfDone)
	frame()
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: rowOfDone})
	act := frame()
	if act.file.folder != "Done" || len(act.file.ids) != 1 || act.file.ids[0] != "a" {
		t.Errorf("dropped on a row of Done, filed %+v, want a into Done", act.file)
	}
}
