package gioterm

import "testing"

// The cursor is composed into its row's image, so a cursor that stays put
// costs nothing: the row is only recomposed when the cursor moves, and then
// the row it left comes with it.
func TestRowsRecomposeForTheCursorOnlyWhenItMoves(t *testing.T) {
	r, err := NewRenderer(RenderRows, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(40, 6, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	vt.Write([]byte("hello"))
	drawFrame(t, r, vt)
	if _, redrawn := drawFrame(t, r, vt); redrawn != 0 {
		t.Errorf("with nothing changed, redrew %d rows, want none", redrawn)
	}
	vt.Write([]byte("!"))
	if _, redrawn := drawFrame(t, r, vt); redrawn != 1 {
		t.Errorf("with the cursor moved along its row, redrew %d rows, want that row", redrawn)
	}
	vt.Write([]byte("\r\n"))
	if _, redrawn := drawFrame(t, r, vt); redrawn != 2 {
		t.Errorf("with the cursor moved to the next row, redrew %d rows, want the row it left and the one it is on", redrawn)
	}
}
