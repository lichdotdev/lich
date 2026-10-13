package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// TestDialogDropdownAnchorsAtItsTrigger proves a dropdown inside a dialog
// opens on its trigger. The dialog is drawn deferred, so the window's Root is
// no ancestor of its content and never sees a press there: unless the dialog
// reports presses to the Root, the panel is placed off the Root's stale
// pointer instead of the trigger.
func TestDialogDropdownAnchorsAtItsTrigger(t *testing.T) {
	var (
		router input.Router
		ops    op.Ops
		root   Root
		state  DialogState
		panel  DropdownPanelState
	)
	th := testTheme()
	now := time.Unix(0, 0)
	frame := func() {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(dialogTestWindow), Source: router.Source()}
		root.Layout(gtx, func(gtx C) D {
			d := th.Dialog(&state, true, "Pick", "")
			d.Root = &root
			d.Body = func(gtx C) D {
				pop := th.DropdownPanel(&root, &panel, 100, func(gtx C, th *Theme) D { return D{Size: image.Pt(100, 40)} })
				return pop.Layout(gtx, func(gtx C) D { return D{Size: image.Pt(200, 36)} })
			}
			d.Layout(gtx)
			return D{Size: gtx.Constraints.Max}
		})
		router.Frame(&ops)
	}
	frame()
	now = now.Add(dialogTransition)
	frame()
	// The body sits under the header, centered: probe down the middle column
	// for the trigger.
	for y := 0; y < dialogTestWindow.Y && !panel.Opened(); y += 4 {
		at := image.Pt(dialogTestWindow.X/2-60, y)
		pos := f32.Pt(float32(at.X), float32(at.Y))
		router.Queue(
			pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pos},
			pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pos},
		)
		frame()
		if panel.Opened() && !at.In(panel.anchor) {
			t.Fatalf("pressed the trigger at %v, the panel anchored at %v", at, panel.anchor)
		}
	}
	if !panel.Opened() {
		t.Fatal("no press opened the dropdown")
	}
}
