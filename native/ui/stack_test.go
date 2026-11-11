package ui

import (
	"image"
	"testing"

	"gioui.org/layout"
)

func sizedBox(w, h int) layout.Widget {
	return func(gtx C) D { return D{Size: image.Pt(w, h)} }
}

func TestStackAndRowPutGapsBetweenTheirChildren(t *testing.T) {
	if d := layoutAt(Stack(2, sizedBox(10, 10), sizedBox(10, 10)), 0, 100); d.Size.Y != 28 {
		t.Errorf("stack height = %d, want 10+8+10", d.Size.Y)
	}
	if d := layoutAt(Row(2, sizedBox(10, 10), sizedBox(10, 10)), 0, 100); d.Size.X != 28 {
		t.Errorf("row width = %d, want 10+8+10", d.Size.X)
	}
}

func TestColumnsShareTheWidthEvenly(t *testing.T) {
	var got []image.Point
	probe := func(gtx C) D {
		got = append(got, image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Max.X))
		return D{Size: image.Pt(1, 1)}
	}
	layoutAt(Columns(0, probe, probe), 200, 200)
	if len(got) != 2 || got[0] != image.Pt(0, 100) || got[1] != got[0] {
		t.Errorf("column constraints (min, max) = %v, want (0, 100) each", got)
	}
}

func TestSpanningTakesTheMinimum(t *testing.T) {
	if d := layoutAt(Spanning(sizedBox(10, 10)), 200, 300); d.Size.X != 200 {
		t.Errorf("width = %d, want the Flexed's 200", d.Size.X)
	}
}
