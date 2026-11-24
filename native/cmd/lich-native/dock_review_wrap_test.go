package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/diffview"
)

func TestANarrowCardGrowsByTheRowsItsLongLinesWrapInto(t *testing.T) {
	long := strings.Repeat("word ", 40)
	f := onlyFile(t, "diff --git a/a.go b/a.go\nindex 1111111..2222222 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,2 @@\n one\n-two\n+"+long+"\n")
	p, err := newReviewPanel(nil, &ui.Root{}, nil, "monospace", 12, func() {}, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	p.th = &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	c := newFileCard(p, f, false, 0)
	view := c.ensureView(p)
	var ops op.Ops
	height := func(width int) int {
		gtx := layout.Context{Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(width, 4000)}}
		return c.wrappedHeight(gtx, p, view, diffview.Frame{Theme: p.th, Code: p.code, Patch: c.patch, Wrap: true})
	}
	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	rows := diffRows(c.shown, false, nil) * p.cellHeight(gtx)
	if wide := height(4000); wide != rows {
		t.Errorf("wide card = %dpx, want %dpx: nothing should wrap", wide, rows)
	}
	if narrow := height(300); narrow <= rows {
		t.Errorf("300px card = %dpx, want taller than its %dpx unwrapped rows", narrow, rows)
	}
	if again := height(4000); again != rows {
		t.Errorf("back to wide = %dpx, want %dpx: the measure kept a stale width", again, rows)
	}
}
