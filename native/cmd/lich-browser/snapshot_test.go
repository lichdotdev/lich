package main

import (
	"image/color"
	"testing"
)

func TestAnARGB32ImageReadsAsTheColorsCairoStored(t *testing.T) {
	// Two pixels a row and a stride with padding, as cairo aligns rows: a red
	// one and a half-covered blue one (premultiplied), stored B, G, R, A.
	pixels := []byte{
		0x00, 0x00, 0xff, 0xff, 0x80, 0x00, 0x00, 0x80, 0xee, 0xee,
		0x00, 0xff, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0xee, 0xee,
	}
	img := fromARGB32(pixels, 2, 2, 10)
	for _, c := range []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, color.RGBA{R: 0xff, A: 0xff}},
		{1, 0, color.RGBA{B: 0x80, A: 0x80}},
		{0, 1, color.RGBA{G: 0xff, A: 0xff}},
		{1, 1, color.RGBA{A: 0xff}},
	} {
		if got := img.RGBAAt(c.x, c.y); got != c.want {
			t.Errorf("pixel %d,%d = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}
