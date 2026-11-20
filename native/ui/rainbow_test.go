package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// sums is the red and green light of img's columns from..to.
func sums(img *image.RGBA, from, to int) (r, g int) {
	for x := from; x < to; x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			c := img.RGBAAt(x, y)
			r, g = r+int(c.R), g+int(c.G)
		}
	}
	return r, g
}

// assertRainbow checks that a drawing width wide starts red and is green
// two fifths across, where the hues put it.
func assertRainbow(t *testing.T, img *image.RGBA, width int) {
	t.Helper()
	r, g := sums(img, 0, width/10)
	if r <= g {
		t.Errorf("the left end holds red %d and green %d, want it red", r, g)
	}
	r, g = sums(img, width*35/100, width*45/100)
	if g <= r {
		t.Errorf("two fifths across holds red %d and green %d, want it green", r, g)
	}
}

func TestARainbowTextRunsFromRedOnTheLeft(t *testing.T) {
	th := testTheme()
	var width int
	ops := tabsFrame(new(input.Router), time.Unix(0, 0), func(gtx C) D {
		d := th.Text(TextBase, "MMMMMMMMMMMMMMMM").Rainbow().Layout(gtx)
		width = d.Size.X
		return d
	})
	assertRainbow(t, tabsRender(t, ops), width)
}

func TestARainbowIconIsFilledInTheRainbow(t *testing.T) {
	th := testTheme()
	var width int
	ops := tabsFrame(new(input.Router), time.Unix(0, 0), func(gtx C) D {
		d := th.RainbowIcon(gtx, icons.Lucide("square"), 40)
		width = d.Size.X
		return d
	})
	assertRainbow(t, tabsRender(t, ops), width)
}
