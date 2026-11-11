package ui

import (
	"image"
	"testing"
)

func TestStatWrapsItsWordsBesideTheGlyph(t *testing.T) {
	th := testTheme()
	const side = 14
	glyph := func(C) D { return D{Size: image.Pt(side, side)} }
	words := "Two checks failed on this branch and one is still running"
	stat := func(maxX int) D {
		return layoutAt(func(gtx C) D { return th.Stat(gtx, glyph, words, th.Foreground) }, 0, maxX)
	}
	wide, narrow := stat(2000), stat(120)
	if narrow.Size.X > 120 {
		t.Fatalf("width = %d, want it kept within the 120 it was given", narrow.Size.X)
	}
	if narrow.Size.Y <= wide.Size.Y {
		t.Fatalf("height = %d narrow, %d wide, want the words wrapped onto more lines", narrow.Size.Y, wide.Size.Y)
	}
	if wide.Size.X <= side+int(Space(1.5)) {
		t.Fatalf("width = %d, want the words after the glyph and its gap", wide.Size.X)
	}
}
