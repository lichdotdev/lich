package main

import (
	"image"
	"slices"
	"testing"
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

func TestARoundedCutLeavesThePageItsCornersAndCoversTheRest(t *testing.T) {
	r := image.Rect(10, 20, 110, 60)
	cut := roundedCut(r, 8, 0)
	covered := func(p image.Point) bool {
		return slices.ContainsFunc(cut, p.In)
	}
	for _, p := range []image.Point{{10, 20}, {109, 20}} {
		if covered(p) {
			t.Fatalf("top corner pixel %v is cut: a rounded corner leaves it to the page", p)
		}
	}
	for _, p := range []image.Point{{10, 59}, {109, 59}, {60, 20}, {10, 40}, {60, 40}} {
		if !covered(p) {
			t.Fatalf("pixel %v is not cut: only the top corners are rounded", p)
		}
	}
}

func TestTheSameRingsMakeTheSameScript(t *testing.T) {
	rings := []pageRing{{X: 1, Y: 2, W: 30, H: 40, Radius: 8, Width: 1.5, Dash: 5, Fill: 0x1a, March: true}}
	if ringsScript(rings) != ringsScript(slices.Clone(rings)) {
		t.Fatal("the same rings made two scripts: the page would be told of a change that is none")
	}
	if ringsScript(nil) == ringsScript(rings) {
		t.Fatal("no rings made the script of a ring: the page would keep its outline")
	}
}
