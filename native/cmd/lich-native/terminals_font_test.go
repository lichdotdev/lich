package main

import (
	"testing"

	"gioui.org/unit"
)

type sizedRenderer struct {
	fixedCells
	size unit.Sp
}

func (r *sizedRenderer) SetSize(size unit.Sp) { r.size = size }

// A font size step keeps each terminal's renderer, faces and all, and only
// resizes it: building a new one loads the family's faces again inside the
// frame the step lands in.
func TestFontSizeStepResizesTheRenderersItHas(t *testing.T) {
	r := &sizedRenderer{size: 13}
	ts := &terminals{family: "Mono", size: 13, byID: map[string]*termView{"s": {id: "s", r: r}}, invalidate: func() {}}
	if err := ts.setFont("Mono", 15); err != nil {
		t.Fatal(err)
	}
	if ts.byID["s"].r != r {
		t.Fatal("a size step replaced the terminal's renderer")
	}
	if r.size != 15 || ts.size != 15 {
		t.Fatalf("renderer at %v, terminals at %v; want both at 15", r.size, ts.size)
	}
}
