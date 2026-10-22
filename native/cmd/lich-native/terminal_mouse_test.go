package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/native/gioterm"
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
		if send, copied := v.pointer(e); send != nil || copied != "" {
			t.Fatalf("%v sent %q, copied %q mid-drag", e.Kind, send, copied)
		}
	}
	send, copied := v.pointer(ptr(pointer.Release, at(4, 0), 0, 0))
	if send != nil || copied != "hello" {
		t.Fatalf("release sent %q, copied %q; want nothing sent and %q copied", send, copied, "hello")
	}
	// A plain click drops the selection and copies nothing.
	v.pointer(ptr(pointer.Press, at(8, 0), pointer.ButtonPrimary, 0))
	if _, copied := v.pointer(ptr(pointer.Release, at(8, 0), 0, 0)); copied != "" {
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
			send, copied := v.pointer(c.e)
			if string(send) != c.want || copied != "" {
				t.Fatalf("sent %q, copied %q; want %q sent", send, copied, c.want)
			}
		})
	}
}

func TestShiftSelectsUnderMouseTracking(t *testing.T) {
	v := newMouseTestView(t, "hello world\x1b[?1000h\x1b[?1006h")
	v.pointer(ptr(pointer.Press, from(6, 0), pointer.ButtonPrimary, key.ModShift))
	v.pointer(ptr(pointer.Drag, at(10, 0), pointer.ButtonPrimary, key.ModShift))
	// Shift may be let go before the button: the drag it started stays a selection.
	send, copied := v.pointer(ptr(pointer.Release, at(10, 0), 0, 0))
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
			if send, _ := v.pointer(e); string(send) != c.want {
				t.Fatalf("sent %q, want %q", send, c.want)
			}
		})
	}
}

func TestTouchpadDeltasUnderACellStillScroll(t *testing.T) {
	v := newMouseTestView(t, "\x1b[?1049h")
	// A touchpad sends a stream of deltas a few pixels each, never a whole row.
	var sent string
	for range 4 {
		e := pointer.Event{Kind: pointer.Scroll, Position: at(0, 0), Scroll: f32.Pt(0, mouseTestCellH/4)}
		send, _ := v.pointer(e)
		sent += string(send)
	}
	if sent != "\x1b[B" {
		t.Fatalf("four quarter-row deltas sent %q, want one row down", sent)
	}
}

func TestShortcutClickOnAURLOpensItAndReachesNothingElse(t *testing.T) {
	// ?1000h: the program tracks the mouse, as Claude Code does.
	v := newMouseTestView(t, "\x1b[?1000hdocs at https://lich.dev/docs ok")
	url, took := v.linkClick(ptr(pointer.Press, at(20, 0), pointer.ButtonPrimary, key.ModShortcut))
	if url != "https://lich.dev/docs" || !took {
		t.Fatalf("Ctrl/Cmd+click on the URL = %q, %v", url, took)
	}
	if url, took := v.linkClick(ptr(pointer.Release, at(20, 0), 0, key.ModShortcut)); url != "" || !took {
		t.Fatalf("the click's release = %q, %v; want it kept from the program and nothing reopened", url, took)
	}
	if _, took := v.linkClick(ptr(pointer.Press, at(20, 0), pointer.ButtonPrimary, 0)); took {
		t.Fatal("a plain click was taken; only Ctrl/Cmd+click opens")
	}
	if _, took := v.linkClick(ptr(pointer.Press, at(2, 0), pointer.ButtonPrimary, key.ModShortcut)); took {
		t.Fatal("Ctrl/Cmd+click off any link was taken")
	}
}

func TestTheLinkUnderThePointerIsHoveredUntilItLeaves(t *testing.T) {
	v := newMouseTestView(t, "docs at https://lich.dev/docs ok")
	v.track(ptr(pointer.Move, at(20, 0), 0, 0))
	if l, ok := v.hoveredLink(); !ok || l.URL != "https://lich.dev/docs" {
		t.Fatalf("hovered over the URL = %+v, %v", l, ok)
	}
	v.track(ptr(pointer.Move, at(2, 0), 0, 0))
	if _, ok := v.hoveredLink(); ok {
		t.Fatal("hovered a link off any link")
	}
	v.track(ptr(pointer.Move, at(20, 0), 0, 0))
	v.track(ptr(pointer.Leave, at(20, 0), 0, 0))
	if _, ok := v.hoveredLink(); ok {
		t.Fatal("a link stayed hovered after the pointer left the grid")
	}
}

// A field elsewhere keeps the keyboard until something takes it: a press on
// the terminal is what hands it back.
func TestPressOnTheTerminalTakesTheKeyboard(t *testing.T) {
	v := newMouseTestView(t, "hello")
	v.tag = new(int)
	field := new(int)
	ts := &terminals{}
	var router input.Router
	frame := func(before func(C)) {
		ops := new(op.Ops)
		gtx := layout.Context{Ops: ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(800, 480))}
		if before != nil {
			before(gtx)
		}
		gtx.Event(key.FocusFilter{Target: field})
		event.Op(ops, field)
		ts.input(gtx, v, false)
		area := clip.Rect{Max: image.Pt(800, 480)}.Push(ops)
		event.Op(ops, v.tag)
		area.Pop()
		router.Frame(ops)
	}
	frame(nil)
	frame(func(gtx C) { gtx.Execute(key.FocusCmd{Tag: field}) })
	router.Queue(ptr(pointer.Press, at(1, 0), pointer.ButtonPrimary, 0), ptr(pointer.Release, at(1, 0), 0, 0))
	frame(nil)
	frame(nil)
	if !router.Source().Focused(v.tag) {
		t.Error("a press on the terminal left the keyboard on the field")
	}
}
