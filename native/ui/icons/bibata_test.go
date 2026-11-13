package icons

import "testing"

func TestBibataArrowKeepsItsViewBox(t *testing.T) {
	segs := BibataArrow().shapes[0].segs
	minX, maxX, minY, maxY := float32(BibataArrowBox), float32(0), float32(BibataArrowBox), float32(0)
	for _, s := range segs {
		if s.op == 'Z' {
			continue
		}
		minX, maxX = min(minX, s.to.X), max(maxX, s.to.X)
		minY, maxY = min(minY, s.to.Y), max(maxY, s.to.Y)
	}
	// the extremes of left_ptr.svg's path
	want := [4]float32{45.4999, 210.753, 23.1213, 231.884}
	for i, got := range [4]float32{minX, maxX, minY, maxY} {
		if d := got - want[i]; d > 0.01 || d < -0.01 {
			t.Fatalf("bounds %v, want %v", [4]float32{minX, maxX, minY, maxY}, want)
		}
	}
}
