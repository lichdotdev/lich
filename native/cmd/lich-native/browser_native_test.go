package main

import (
	"image"
	"image/color"
	"slices"
	"testing"

	"gioui.org/f32"
)

func TestAPageIsCutWhereALayerCoversItAndNowhereElse(t *testing.T) {
	page := image.Rect(100, 50, 900, 650)
	own := []image.Rectangle{image.Rect(10, 10, 60, 30)}
	covers := []image.Rectangle{
		image.Rect(700, 0, 1000, 700), // a dock sliding in from the right, past the page's edge
		image.Rect(0, 0, 80, 700),     // a sidebar that stops short of the page
	}
	got := pageCuts(page, own, covers)
	want := []image.Rectangle{image.Rect(10, 10, 60, 30), image.Rect(600, 0, 800, 600)}
	if !slices.Equal(got, want) {
		t.Fatalf("cuts %v, want the page's own and the dock's part over the page, local to it: %v", got, want)
	}
}

func TestAPageWithNoCoverKeepsItsOwnCutsOnly(t *testing.T) {
	own := []image.Rectangle{image.Rect(0, 0, 5, 5)}
	if got := pageCuts(image.Rect(0, 0, 100, 100), own, nil); !slices.Equal(got, own) {
		t.Fatalf("cuts %v, want %v", got, own)
	}
}

func TestTheSameOverlayMakesTheSameScript(t *testing.T) {
	rings := []pageRing{{X: 1, Y: 2, W: 30, H: 40, Radius: 8, Width: 1.5, Dash: 5, Fill: 0x1a, March: true}}
	if overlayScript(pageOverlay{Rings: rings}) != overlayScript(pageOverlay{Rings: slices.Clone(rings)}) {
		t.Fatal("the same rings made two scripts: the page would be told of a change that is none")
	}
	if overlayScript(pageOverlay{}) == overlayScript(pageOverlay{Rings: rings}) {
		t.Fatal("no rings made the script of a ring: the page would keep its outline")
	}
}

func TestTheOverlayCarriesTheSketchAndHoldsTheWheelUnderIt(t *testing.T) {
	n := browserNotes{strokes: []sketchStroke{
		{ink: color.NRGBA{R: 0xff, A: 0xff}, points: []f32.Point{{X: 10, Y: 20}, {X: 30, Y: 40}}},
		{ink: color.NRGBA{B: 0xff, A: 0xff}, points: []f32.Point{{X: 1, Y: 1}, {X: 5, Y: 5}}, doomed: true},
	}}
	o := pageOverlayOf(&n)
	if !o.Hold {
		t.Fatal("strokes on the page leave the wheel free: the page would scroll from under them")
	}
	if len(o.Strokes) != 2 || o.Strokes[0].Ink != "#ff0000" || o.Strokes[0].Opacity != 1 {
		t.Fatalf("strokes %+v, want the red one whole first", o.Strokes)
	}
	if got := o.Strokes[1].Opacity; got != sketchDoomedOpacity {
		t.Fatalf("a stroke the eraser touched shows at %v, want %v", got, sketchDoomedOpacity)
	}
	if got := o.Strokes[0].Lines[0]; !slices.Equal(got, []float32{10, 20, 30, 40}) {
		t.Fatalf("the red stroke's line is %v, want its points as x, y pairs", got)
	}
	if pageOverlayOf(&browserNotes{}).Hold {
		t.Fatal("a page with no strokes on it holds the wheel")
	}
}
