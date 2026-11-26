package ui

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// A surface's motion draws it in an opacity layer and pops it when the
// motion ends; the shadow must not vanish when it does.
func TestAShadowLooksTheSameInAndOutOfAMotion(t *testing.T) {
	under := func(layered bool) color.RGBA {
		size := image.Pt(200, 200)
		win, err := headless.NewWindow(size.X, size.Y)
		if err != nil {
			t.Skipf("no headless GPU: %v", err)
		}
		defer win.Release()
		ops := new(op.Ops)
		paint.Fill(ops, color.NRGBA{R: 60, G: 60, B: 64, A: 255})
		gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(size)}
		layer := paint.PushOpacity(ops, 0.999)
		if !layered {
			layer.Pop()
		}
		op.Offset(image.Pt(50, 50)).Add(ops)
		BoxShadow{Y: 8, Blur: 24, Alpha: 0.22}.Paint(gtx, image.Pt(100, 60), 10)
		if layered {
			layer.Pop()
		}
		if err := win.Frame(ops); err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rectangle{Max: size})
		if err := win.Screenshot(img); err != nil {
			t.Fatal(err)
		}
		return img.RGBAAt(100, 115)
	}
	ground := color.RGBA{R: 60, G: 60, B: 64, A: 255}
	still, moving := under(false), under(true)
	if still == ground {
		t.Errorf("the shadow under the box's edge is the ground %v once no motion draws it", still)
	}
	if still != moving {
		t.Errorf("the shadow under the box's edge is %v still and %v in a motion, want the same", still, moving)
	}
}
