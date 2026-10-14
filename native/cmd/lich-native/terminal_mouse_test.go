package main

import (
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/omartelo/lich/native/gioterm"
)

const mouseTestCellW, mouseTestCellH = 10, 20

func newMouseTestView(t *testing.T, output string) *termView {
	t.Helper()
	vt, err := gioterm.New(80, 24, 10, 20, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	if err := vt.Resize(80, 24, mouseTestCellW, mouseTestCellH); err != nil {
		t.Fatal(err)
	}
	vt.Write([]byte(output))
	return &termView{vt: vt}
}

// at is a pixel in the right half of cell (col, row), where a drag ends
// taking the cell; from is one in its left half, where a drag starts taking it.
func at(col, row int) f32.Point {
	return f32.Pt(float32(col*mouseTestCellW+8), float32(row*mouseTestCellH+10))
}

func from(col, row int) f32.Point {
	return f32.Pt(float32(col*mouseTestCellW+2), float32(row*mouseTestCellH+10))
}

func ptr(kind pointer.Kind, pos f32.Point, buttons pointer.Buttons, mods key.Modifiers) pointer.Event {
	return pointer.Event{Kind: kind, Position: pos, Buttons: buttons, Modifiers: mods, Time: time.Second}
}

func TestDragSelectsAndCopiesOnRelease(t *testing.T) {
	v := newMouseTestView(t, "hello world")
	steps := []pointer.Event{
		ptr(pointer.Press, from(0, 0), pointer.ButtonPrimary, 0),
		ptr(pointer.Drag, at(4, 0), pointer.ButtonPrimary, 0),
	}
	for _, e := range steps {
		if send, copied := v.pointer(e, mouseTestCellH); send != nil || copied != "" {
			t.Fatalf("%v sent %q, copied %q mid-drag", e.Kind, send, copied)
		}
	}
	send, copied := v.pointer(ptr(pointer.Release, at(4, 0), 0, 0), mouseTestCellH)
	if send != nil || copied != "hello" {
		t.Fatalf("release sent %q, copied %q; want nothing sent and %q copied", send, copied, "hello")
	}
	// A plain click drops the selection and copies nothing.
	v.pointer(ptr(pointer.Press, at(8, 0), pointer.ButtonPrimary, 0), mouseTestCellH)
	if _, copied := v.pointer(ptr(pointer.Release, at(8, 0), 0, 0), mouseTestCellH); copied != "" {
		t.Fatalf("a click copied %q", copied)
	}
	if got := v.vt.SelectionText(); got != "" {
		t.Fatalf("selection %q left after a click", got)
	}
}

func TestMouseGoesToTheProgramThatTracksIt(t *testing.T) {
	v := newMouseTestView(t, "hello world\x1b[?1002h\x1b[?1006h")
	cases := []struct {
		name string
		e    pointer.Event
		want string
	}{
		{"press", ptr(pointer.Press, at(2, 1), pointer.ButtonPrimary, 0), "\x1b[<0;3;2M"},
		{"drag", ptr(pointer.Drag, at(3, 1), pointer.ButtonPrimary, 0), "\x1b[<32;4;2M"},
		{"release", ptr(pointer.Release, at(3, 1), 0, 0), "\x1b[<0;4;2m"},
		{"right press", ptr(pointer.Press, at(3, 1), pointer.ButtonSecondary, 0), "\x1b[<2;4;2M"},
		{"right release", ptr(pointer.Release, at(3, 1), 0, 0), "\x1b[<2;4;2m"},
		{"wheel down two rows", pointer.Event{Kind: pointer.Scroll, Position: at(3, 1), Scroll: f32.Pt(0, 2*mouseTestCellH)}, "\x1b[<65;4;2M\x1b[<65;4;2M"},
		{"wheel up", pointer.Event{Kind: pointer.Scroll, Position: at(3, 1), Scroll: f32.Pt(0, -mouseTestCellH)}, "\x1b[<64;4;2M"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			send, copied := v.pointer(c.e, mouseTestCellH)
			if string(send) != c.want || copied != "" {
				t.Fatalf("sent %q, copied %q; want %q sent", send, copied, c.want)
			}
		})
	}
}

func TestShiftSelectsUnderMouseTracking(t *testing.T) {
	v := newMouseTestView(t, "hello world\x1b[?1000h\x1b[?1006h")
	v.pointer(ptr(pointer.Press, from(6, 0), pointer.ButtonPrimary, key.ModShift), mouseTestCellH)
	v.pointer(ptr(pointer.Drag, at(10, 0), pointer.ButtonPrimary, key.ModShift), mouseTestCellH)
	// Shift may be let go before the button: the drag it started stays a selection.
	send, copied := v.pointer(ptr(pointer.Release, at(10, 0), 0, 0), mouseTestCellH)
	if send != nil || copied != "world" {
		t.Fatalf("sent %q, copied %q; want %q copied", send, copied, "world")
	}
}

func TestWheelMovesTheCursorOnTheAlternateScreen(t *testing.T) {
	v := newMouseTestView(t, "\x1b[?1049h")
	cases := []struct {
		name   string
		scroll float32
		want   string
	}{
		{"down two rows", 2 * mouseTestCellH, "\x1b[B\x1b[B"},
		{"up one row", -mouseTestCellH, "\x1b[A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := pointer.Event{Kind: pointer.Scroll, Position: at(0, 0), Scroll: f32.Pt(0, c.scroll)}
			if send, _ := v.pointer(e, mouseTestCellH); string(send) != c.want {
				t.Fatalf("sent %q, want %q", send, c.want)
			}
		})
	}
}
