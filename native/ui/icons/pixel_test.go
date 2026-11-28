package icons

import (
	"image/color"
	"io/fs"
	"strings"
	"testing"
)

// Every pixel mark renders its grid in whole blocks: at twice its size each
// of its pixels is a 2×2 block in the ink, and at a size between two
// multiples it stays at the smaller one, centered, with nothing smeared
// between blocks.
func TestPixelArtScalesByWholeBlocks(t *testing.T) {
	files, err := fs.Glob(pixelFiles, "lich/*.png")
	if err != nil || len(files) == 0 {
		t.Fatalf("no pixel marks: %v", err)
	}
	ink := color.NRGBA{R: 200, G: 10, B: 10, A: 255}
	for _, file := range files {
		a := Lich(strings.TrimSuffix(strings.TrimPrefix(file, "lich/"), ".png"))
		w := a.src.Bounds().Dx()
		for _, px := range []int{2 * w, 2*w + 5} {
			img := a.render(px, ink)
			off := (px - 2*w) / 2
			for y := range px {
				for x := range px {
					got := img.NRGBAAt(x, y)
					sx, sy := (x-off)/2, (y-off)/2
					inside := x >= off && y >= off && sx < w && sy < w
					want := inside && a.src.NRGBAAt(sx, sy).A != 0
					if want && got != ink || !want && got.A != 0 {
						t.Fatalf("%s at %dpx: pixel (%d,%d) = %v, want ink %v", file, px, x, y, got, want)
					}
				}
			}
		}
	}
}
