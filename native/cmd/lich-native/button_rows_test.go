package main

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/ui"
)

// buttonRowGap is how far apart two buttons on one line may sit and still be
// one row of buttons, the gap-2 a row of actions uses with room to spare.
const buttonRowGap = 24

// mixedButtonRows names every pair of buttons side by side whose heights
// differ, which native/DESIGN.md › Buttons calls a bug.
func mixedButtonRows(nodes []input.SemanticNode) []string {
	var buttons []image.Rectangle
	for _, n := range nodes {
		if n.Desc.Class == semantic.Button && !n.Desc.Bounds.Empty() {
			buttons = append(buttons, n.Desc.Bounds)
		}
	}
	var mixed []string
	for i, a := range buttons {
		for _, b := range buttons[i+1:] {
			if sideBySide(a, b) && a.Dy() != b.Dy() {
				mixed = append(mixed, fmt.Sprintf("%v (h %d) beside %v (h %d)", a, a.Dy(), b, b.Dy()))
			}
		}
	}
	return mixed
}

// sideBySide is whether a and b share a centre line and sit within
// buttonRowGap of each other across.
func sideBySide(a, b image.Rectangle) bool {
	ca, cb := (a.Min.Y+a.Max.Y)/2, (b.Min.Y+b.Max.Y)/2
	if abs(ca-cb) > 2 {
		return false
	}
	gap := max(b.Min.X-a.Max.X, a.Min.X-b.Max.X)
	return gap >= 0 && gap <= buttonRowGap
}

func describeMixed(mixed []string) string { return strings.Join(mixed, "\n") }

func TestMixedButtonRowsFindsAMixedRow(t *testing.T) {
	var a, b ui.Clickable
	r := newFrameRig(image.Pt(400, 200), func(gtx C, th *ui.Theme) {
		small := th.Button(&a, "Cancel")
		small.Size = ui.SizeXS
		layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(small.Layout), ui.Gap(2), layout.Rigid(th.Button(&b, "Reply").Layout))
	})
	r.allowMixed = true
	r.frame()
	if m := mixedButtonRows(r.nodes()); len(m) != 1 {
		t.Fatalf("a 24px button beside a 32px one: %d mixed pairs, want 1\n%s", len(m), r.drawn())
	}
}
