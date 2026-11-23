package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// The account panel stands on the account row's left and right edges, at any
// width the sidebar is dragged to.
func TestTheAccountPanelIsAsWideAsTheSidebarsRow(t *testing.T) {
	for _, row := range []int{224, 288, 400} {
		_, m, _ := profileFooterRig(t)
		a, _ := m.focusedAccount()
		a.plan, a.loaded = quotaPlan(backend.QuotaWindow{Label: "Session", Seconds: 5 * hour, Percent: 28}), true
		const inset = 16
		var pf profileFooter
		var root ui.Root
		var act sidebarAction
		r := newFrameRig(image.Pt(row+2*inset, 700), nil)
		r.draw = func(gtx C, th *ui.Theme) {
			root.Layout(gtx, func(gtx C) D {
				defer op.Offset(image.Pt(inset, 0)).Push(gtx.Ops).Pop()
				gtx.Constraints.Max.X = row
				pf.layout(gtx, th, &root, m, a, &act)
				return D{Size: gtx.Constraints.Max}
			})
		}
		r.frame()
		r.queueClick(f32.Pt(float32(inset+row/2), 20))
		r.frame()
		r.frame()
		if !pf.panel.Opened() {
			t.Fatalf("row %d: the click missed the account row", row)
		}
		if box := pf.panel.Box(); box.Min.X != inset || box.Dx() != row {
			t.Errorf("row %d: the panel spans x %d..%d, want %d..%d", row, box.Min.X, box.Max.X, inset, inset+row)
		}
		r.now = r.now.Add(time.Second)
		r.frame()
		writeShot(t, r, fmt.Sprintf("account-panel-%d.png", row))
	}
}
