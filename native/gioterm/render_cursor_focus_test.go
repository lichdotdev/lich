package gioterm

import (
	"fmt"
	"image"
	"testing"
)

// A terminal that does not have the keyboard draws its cursor as a 1px
// outline, in every renderer, and a focus change repaints it.
func TestUnfocusedCursorIsAnOutline(t *testing.T) {
	for _, kind := range []string{RenderRows, RenderAtlas, RenderPaths} {
		t.Run(kind, func(t *testing.T) {
			r, err := NewRenderer(kind, "monospace", 14)
			if err != nil {
				t.Fatal(err)
			}
			vt, err := New(40, 6, 8, 16, func([]byte) {})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(vt.Close)
			vt.Write([]byte("hi"))
			paint := func(focused bool) string {
				digest, _ := drawFocused(t, r, vt, focused)
				return fmt.Sprint(digest, rowPixels(r))
			}
			focused, hollow := paint(true), paint(false)
			if focused == hollow {
				t.Fatal("focused and unfocused cursors paint the same")
			}
			if paint(true) != focused {
				t.Error("a focused cursor drawn after an outline one paints differently from before")
			}
		})
	}
}

// The outline leaves the cell's own paint inside it, the block fills it.
func TestRowsUnfocusedCursorKeepsTheCellInside(t *testing.T) {
	r, err := NewRenderer(RenderRows, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(40, 6, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	vt.Write([]byte("hi"))
	drawFocused(t, r, vt, true)
	rr := r.(*rowRenderer)
	cw, ch := rr.r.CellW(), rr.r.CellH()
	s, _ := vt.Update()
	ink, ground := rgba(s.s.fg), rgba(s.s.bg)
	cursor := image.Rect(2*cw, 0, 3*cw, ch)
	px := func(p image.Point) [4]uint8 {
		c := rr.imgs[0].RGBAAt(p.X, p.Y)
		return [4]uint8{c.R, c.G, c.B, c.A}
	}
	want := func(c [4]uint8) [4]uint8 { return c }
	inside := cursor.Min.Add(image.Pt(cw/2, ch/2))
	if got := px(inside); got != want([4]uint8{ink.R, ink.G, ink.B, ink.A}) {
		t.Fatalf("focused cursor center is %v, want the block in %v", got, ink)
	}
	if _, redrawn := drawFocused(t, r, vt, false); redrawn != 1 {
		t.Errorf("losing focus redrew %d rows, want the cursor's", redrawn)
	}
	if got := px(inside); got != want([4]uint8{ground.R, ground.G, ground.B, ground.A}) {
		t.Errorf("unfocused cursor center is %v, want the terminal's own %v", got, ground)
	}
	for _, edge := range []image.Point{cursor.Min, {cursor.Max.X - 1, 0}, {cursor.Min.X, ch - 1}, {cursor.Max.X - 1, ch - 1}, {cursor.Min.X, ch / 2}, {cursor.Max.X - 1, ch / 2}} {
		if got := px(edge); got != want([4]uint8{ink.R, ink.G, ink.B, ink.A}) {
			t.Errorf("outline at %v is %v, want %v", edge, got, ink)
		}
	}
	if _, redrawn := drawFocused(t, r, vt, false); redrawn != 0 {
		t.Errorf("with the focus unchanged, redrew %d rows, want none", redrawn)
	}
}
