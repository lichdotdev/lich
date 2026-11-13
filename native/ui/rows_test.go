package ui

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/widget"
)

// A hovered panel row lights its whole width, corners included: a rounded
// fill set in from the sides showed its corners against a panel's seams.
func TestPanelRowHoverFillsTheWholeRow(t *testing.T) {
	th, click := testTheme(), new(widget.Clickable)
	const w, h = 200, 40
	rig := newInputRig(t, w, h, func(gtx layout.Context) layout.Dimensions {
		return th.PanelRow(gtx, click, true, Space(2), Space(1.5), func(gtx C, _ *Theme) D { return D{Size: image.Pt(0, h-16)} })
	})
	rig.frame()
	rig.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: inputPointF32(image.Pt(rig.margin+w/2, rig.margin+h/2))})
	rig.frame()
	lit := rig.frame()
	m := rig.margin
	fill := lit.RGBAAt(m+w/2, m+1)
	if fill == rest(rig) {
		t.Fatal("the pointer over the row did not light it")
	}
	for _, p := range []image.Point{{m, m}, {m + w - 1, m}, {m, m + h - 1}, {m + w - 1, m + h - 1}} {
		if lit.RGBAAt(p.X, p.Y) != fill {
			t.Errorf("the hover left %v unlit", p)
		}
	}
}

// rest is the page around the rig's widget, what an unlit pixel shows.
func rest(rig *inputRig) color.RGBA { return rig.frame().RGBAAt(0, 0) }
