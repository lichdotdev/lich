package ui

import (
	"math"
	"testing"

	"gioui.org/f32"
)

func TestTheRingsOutlineWalksItsSidesAndCorners(t *testing.T) {
	r := roundedRect{max: f32.Pt(100, 50), radius: 10}
	if got, want := r.perimeter(), float32(2*80+2*30+2*math.Pi*10); math.Abs(float64(got-want)) > 1e-3 {
		t.Fatalf("perimeter %v, want %v", got, want)
	}
	near := func(p, q f32.Point) bool { return math.Hypot(float64(p.X-q.X), float64(p.Y-q.Y)) < 1e-2 }
	arc := float32(math.Pi / 2 * 10)
	cases := []struct {
		t    float32
		want f32.Point
	}{
		{0, f32.Pt(10, 0)},
		{80, f32.Pt(90, 0)},
		{80 + arc, f32.Pt(100, 10)},
		{80 + arc + 30, f32.Pt(100, 40)},
		{80 + 2*arc + 30, f32.Pt(90, 50)},
		{2*80 + 4*arc + 2*30, f32.Pt(10, 0)},
	}
	for _, c := range cases {
		if got := r.at(c.t); !near(got, c.want) {
			t.Errorf("at(%v) = %v, want %v", c.t, got, c.want)
		}
	}
}
