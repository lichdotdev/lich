package main

import (
	"image"
	"testing"

	"gioui.org/gpu/headless"
)

// renderRig paints r's last frame on a GPU without a window.
func renderRig(t *testing.T, r *frameRig) *image.RGBA {
	t.Helper()
	win, err := headless.NewWindow(r.size.X, r.size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	if err := win.Frame(&r.ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: r.size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

// labelBox is where the last frame drew label.
func labelBox(t *testing.T, r *frameRig, label string) image.Rectangle {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			return n.Desc.Bounds
		}
	}
	t.Fatalf("no %q drawn:\n%s", label, r.drawn())
	return image.Rectangle{}
}

// inkShift is the largest change of any channel of any pixel inside box
// between a and b.
func inkShift(a, b *image.RGBA, box image.Rectangle) uint8 {
	var most uint8
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			ca, cb := a.RGBAAt(x, y), b.RGBAAt(x, y)
			for _, d := range [][2]uint8{{ca.R, cb.R}, {ca.G, cb.G}, {ca.B, cb.B}} {
				most = max(most, max(d[0], d[1])-min(d[0], d[1]))
			}
		}
	}
	return most
}
