package icons

import "testing"

// The mark is scaled from appicon.svg's 1024 units into the sets' 24, so it
// sits where the SVG puts it: x from 225 to 798.8, y from 102.8 to 919.8.
func TestLichMarkFitsTheViewBox(t *testing.T) {
	segs := LichMark().shapes[0].segs
	minX, maxX, minY, maxY := float32(viewBox), float32(0), float32(viewBox), float32(0)
	for _, s := range segs {
		if s.op == 'Z' {
			continue
		}
		minX, maxX = min(minX, s.to.X), max(maxX, s.to.X)
		minY, maxY = min(minY, s.to.Y), max(maxY, s.to.Y)
	}
	want := [4]float32{225 * 24 / 1024.0, 798.8 * 24 / 1024.0, 102.8 * 24 / 1024.0, 919.8 * 24 / 1024.0}
	for i, got := range [4]float32{minX, maxX, minY, maxY} {
		if d := got - want[i]; d > 0.01 || d < -0.01 {
			t.Errorf("bounds %v, want %v", [4]float32{minX, maxX, minY, maxY}, want)
			break
		}
	}
}
