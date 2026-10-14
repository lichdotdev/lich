package gioterm

import (
	"testing"
	"time"
)

const testCellW, testCellH = 10, 20

func newSizedTerminal(t *testing.T) *Terminal {
	t.Helper()
	v, err := New(80, 24, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Resize(80, 24, testCellW, testCellH); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Close)
	return v
}

// cellCenter is the pixel at the middle of cell (col, row).
func cellCenter(col, row int) (x, y float32) {
	return float32(col*testCellW + testCellW/2), float32(row*testCellH + testCellH/2)
}

func TestEncodeMouseReportsOnlyWhatTheProgramAskedFor(t *testing.T) {
	v := newSizedTerminal(t)
	x, y := cellCenter(2, 1)
	if v.MouseTracking() {
		t.Fatal("tracking before the program asked for it")
	}
	if got := v.EncodeMouse(MousePress, MouseLeft, 0, x, y, false); got != nil {
		t.Fatalf("reported %q without tracking", got)
	}
	v.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	if !v.MouseTracking() {
		t.Fatal("no tracking after ?1000h")
	}
	cases := []struct {
		name   string
		action MouseAction
		button MouseButton
		want   string
	}{
		{"press", MousePress, MouseLeft, "\x1b[<0;3;2M"},
		{"release", MouseRelease, MouseLeft, "\x1b[<0;3;2m"},
		{"wheel down", MousePress, MouseWheelDown, "\x1b[<65;3;2M"},
		// ?1000 reports buttons, not motion.
		{"motion", MouseMotion, MouseNoButton, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(v.EncodeMouse(c.action, c.button, 0, x, y, false)); got != c.want {
				t.Fatalf("reported %q, want %q", got, c.want)
			}
		})
	}
}

func TestDragSelectsAndDrawsTheCellsInverted(t *testing.T) {
	v := newSizedTerminal(t)
	v.Write([]byte("hello world"))
	if _, err := v.Update(); err != nil {
		t.Fatal(err)
	}
	x0, y0 := cellCenter(0, 0)
	v.SelectPress(x0, y0, time.Second)
	// Past the middle of the cell: a drag takes a cell once it crosses half of it.
	x, y := float32(4*testCellW+8), float32(testCellH/2)
	v.SelectDrag(x, y)
	v.SelectRelease(x, y)
	if got := v.SelectionText(); got != "hello" {
		t.Fatalf("selected %q, want %q", got, "hello")
	}
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	for col, want := range []bool{true, true, true, true, true, false} {
		if got := s.cells[col].flags&cellInverse != 0; got != want {
			t.Fatalf("cell %d inverted %v, want %v", col, got, want)
		}
	}
	v.ClearSelection()
	if got := v.SelectionText(); got != "" {
		t.Fatalf("selected %q after clearing", got)
	}
}

func TestDoubleClickSelectsTheWord(t *testing.T) {
	v := newSizedTerminal(t)
	v.Write([]byte("hello world"))
	x, y := cellCenter(8, 0)
	start := time.Second
	v.SelectPress(x, y, start)
	v.SelectRelease(x, y)
	v.SelectPress(x, y, start+100*time.Millisecond)
	if got := v.SelectionText(); got != "world" {
		t.Fatalf("selected %q, want %q", got, "world")
	}
}
