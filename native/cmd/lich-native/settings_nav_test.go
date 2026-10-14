package main

import (
	"image"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/fonts"
	"github.com/omartelo/lich/native/ui"
)

// A "1" in Onest has most of its advance to the right of its ink: centering
// the advance drew it 3px left of the pill's middle at 1x.
func TestCountBadgeCentersTheDigitInk(t *testing.T) {
	faces, err := fonts.Faces()
	if err != nil {
		t.Fatal(err)
	}
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(faces)), Sans: fonts.Sans, Surface: ui.Dark.Canvas}
	const side = 40
	win, err := headless.NewWindow(side, side)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	for _, n := range []int{1, 7, 12} {
		ops := new(op.Ops)
		gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(side, side)}}
		d := countBadge(gtx, th, n)
		if err := win.Frame(ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, side, side))
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		ink := image.Rectangle{Min: image.Pt(side, side)}
		for y := range side {
			for x := range side {
				// The amber digit is the only bright red channel; the pill is a
				// 20% tint over black.
				if img.RGBAAt(x, y).R > 120 {
					ink = ink.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		left, right := ink.Min.X, d.Size.X-ink.Max.X
		top, bottom := ink.Min.Y, d.Size.Y-ink.Max.Y
		if abs(left-right) > 1 || abs(top-bottom) > 1 {
			t.Errorf("%d: ink %v in badge %v, margins left %d right %d top %d bottom %d", n, ink, d.Size, left, right, top, bottom)
		}
	}
}

func abs(n int) int { return max(n, -n) }
