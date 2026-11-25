package ui

import (
	"image"
	"math"
	"slices"
	"testing"
)

// A rounded panel's cover takes its ring and every pixel of its rounded
// surface, and leaves the corners outside the curve to the page.
func TestACoverFollowsItsPanelsRoundedRing(t *testing.T) {
	c := Cover{Box: image.Rect(10, 20, 110, 60), Radius: 8, Ring: 1}
	rects := c.Rects()
	outer, r := c.Box.Inset(-1), 9.0
	for y := outer.Min.Y - 2; y < outer.Max.Y+2; y++ {
		for x := outer.Min.X - 2; x < outer.Max.X+2; x++ {
			p := image.Pt(x, y)
			if !p.In(outer) {
				if slices.ContainsFunc(rects, p.In) {
					t.Fatalf("%v, past the ring, is covered", p)
				}
				continue
			}
			// The pixel's center against the nearest corner's circle.
			cx := min(max(float64(x)+0.5, float64(outer.Min.X)+r), float64(outer.Max.X)-r)
			cy := min(max(float64(y)+0.5, float64(outer.Min.Y)+r), float64(outer.Max.Y)-r)
			inside := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) <= r
			if inside && !slices.ContainsFunc(rects, p.In) {
				t.Fatalf("%v, on the panel, is left to the page", p)
			}
		}
	}
	for _, corner := range []image.Point{outer.Min, {outer.Max.X - 1, outer.Min.Y}, {outer.Min.X, outer.Max.Y - 1}, outer.Max.Sub(image.Pt(1, 1))} {
		if slices.ContainsFunc(rects, corner.In) {
			t.Errorf("corner %v, outside the curve, is covered", corner)
		}
	}
}

func TestASquareCoverIsItsBox(t *testing.T) {
	box := image.Rect(0, 0, 40, 30)
	if got := (Cover{Box: box}).Rects(); len(got) != 1 || got[0] != box {
		t.Errorf("rects %v, want %v", got, box)
	}
	if got := (Cover{}).Rects(); len(got) != 0 {
		t.Errorf("an empty cover gives %v", got)
	}
}

func TestRoundedRowsLeaveThePageItsCornersAndCoversTheRest(t *testing.T) {
	r := image.Rect(10, 20, 110, 60)
	cut := RoundedRows(r, 8, 0)
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

// A page squared under a rounded card loses only what lies outside the
// card's corners, all four of them.
func TestRoundedCornersTakeOnlyWhatLiesOutsideTheCurve(t *testing.T) {
	r := image.Rect(0, 0, 100, 50)
	cut := RoundedCorners(r, 8)
	for _, p := range []image.Point{{0, 0}, {99, 0}, {0, 49}, {99, 49}} {
		if !slices.ContainsFunc(cut, p.In) {
			t.Errorf("corner pixel %v stays: the card's rounding leaves it out", p)
		}
	}
	for _, p := range []image.Point{{8, 0}, {0, 8}, {50, 25}, {3, 3}, {96, 46}} {
		if slices.ContainsFunc(cut, p.In) {
			t.Errorf("pixel %v, inside the curve, is cut", p)
		}
	}
}
