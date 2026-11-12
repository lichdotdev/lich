package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// A screen keeps its actions in its top-right corner, where the stack used to
// stand over them.
func TestToastsStandInTheBottomRightCornerOverTheFooter(t *testing.T) {
	const footer = 60
	size := image.Pt(900, 600)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := &ui.Theme{Palette: ui.Dark, Surface: ui.Dark.Canvas, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
	u := &window{theme: th, footerHeight: footer}
	u.toaster.Show(ui.Toast{Title: "Saved", Duration: ui.ToastForever})
	now := time.Now()
	ops := new(op.Ops)
	for range 2 {
		ops.Reset()
		paint.Fill(ops, th.Canvas)
		u.layoutToasts(layout.Context{Ops: ops, Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(size)})
		now = now.Add(time.Second)
	}
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	ground := img.RGBAAt(0, 0)
	drawn := image.Rectangle{}
	for y := range size.Y {
		for x := range size.X {
			if img.RGBAAt(x, y) != ground {
				drawn = drawn.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if drawn.Empty() {
		t.Fatal("no toast drawn")
	}
	if drawn.Min.Y < size.Y/2 || drawn.Min.X < size.X/2 {
		t.Errorf("the toast is drawn at %v, want it in the bottom-right corner, clear of a screen's top-right actions", drawn)
	}
	// The box ends the gutter over the footer; its shadow falls into the gap.
	if box, top := size.Y-footer-int(gutter), size.Y-footer; drawn.Max.Y < box || drawn.Max.Y > top {
		t.Errorf("the toast ends at y=%d, want its box at %d and nothing past %d, the top of a %dpx footer", drawn.Max.Y, box, top, footer)
	}
}
