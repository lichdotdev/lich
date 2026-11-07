package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/ui"
)

// ridingTerminals is a window's terminals with one shown session and the
// window's beat, counting the frames output asks for.
func ridingTerminals(t *testing.T) (*terminals, *ui.Theme, *int) {
	t.Helper()
	frames := 0
	beat := &ui.LoopBeat{}
	ts := &terminals{byID: map[string]*termView{}, shown: map[string]bool{"s": true}, beat: beat, invalidate: func() { frames++ }}
	vt, err := gioterm.New(80, 24, 10, 20, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	tv := &termView{id: "s", vt: vt}
	tv.seeded.Store(true)
	ts.byID["s"] = tv
	th := &ui.Theme{Palette: ui.Dark, Surface: ui.Dark.Background, Beat: beat}
	return ts, th, &frames
}

// spin draws a looping animation at now, as a busy ring does each frame.
func spin(th *ui.Theme, now time.Time) {
	var r input.Router
	gtx := layout.Context{Ops: new(op.Ops), Now: now, Source: r.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	th.Loop(gtx)
}

func TestOutputRidesTheLoopingFrame(t *testing.T) {
	ts, th, frames := ridingTerminals(t)
	ts.onOutput("s", []byte("a"))
	if *frames != 1 {
		t.Fatalf("with nothing looping, output asked for %d frames, want its own", *frames)
	}
	spin(th, time.Now())
	ts.onOutput("s", []byte("b"))
	if *frames != 1 {
		t.Errorf("with a looping frame due, output asked for %d frames, want to ride it", *frames)
	}
	// A beat last drawn a LoopFrame ago: frames run forward, so a later
	// frame cannot pull the beat above back into the past.
	ts.beat = &ui.LoopBeat{}
	th.Beat = ts.beat
	spin(th, time.Now().Add(-ui.LoopFrame))
	ts.onOutput("s", []byte("c"))
	if *frames != 2 {
		t.Errorf("with the looping frame past due, output asked for %d frames, want its own", *frames)
	}
}

func TestAKeystrokesEchoNeverWaits(t *testing.T) {
	ts, th, frames := ridingTerminals(t)
	spin(th, time.Now())
	ts.sent.Store(time.Now().UnixNano())
	ts.onOutput("s", []byte("a"))
	if *frames != 1 {
		t.Errorf("output right after a write to the PTY asked for %d frames, want one at once", *frames)
	}
	ts.sent.Store(time.Now().Add(-echoWindow - time.Millisecond).UnixNano())
	ts.onOutput("s", []byte("b"))
	if *frames != 1 {
		t.Errorf("output past the echo window asked for %d frames, want to ride the loop", *frames)
	}
}

// A program polling the terminal (Claude Code asks for the cursor position
// as it draws) gets its answers written to the PTY all the time; those are
// no one typing, and must not keep output off the looping frame.
func TestAQuerysReplyOpensNoEchoWindow(t *testing.T) {
	ts, th, frames := ridingTerminals(t)
	// Read-only: the writes stop short of the stream this test has none of.
	ts.byID["s"].readOnly.Store(true)
	spin(th, time.Now())
	ts.reply("s", []byte("\x1b[24;80R"))
	ts.onOutput("s", []byte("a"))
	if *frames != 0 {
		t.Errorf("output after a reply asked for %d frames, want to ride the loop", *frames)
	}
	ts.send("s", []byte("k"))
	ts.onOutput("s", []byte("k"))
	if *frames != 1 {
		t.Errorf("output after a keystroke asked for %d frames, want one at once", *frames)
	}
}

func TestEchoOutputInvalidatesUnderItsOwnName(t *testing.T) {
	ts, _, frames := ridingTerminals(t)
	echoes := 0
	ts.invalidateEcho = func() { echoes++ }
	ts.sent.Store(time.Now().UnixNano())
	ts.onOutput("s", []byte("a"))
	if echoes != 1 || *frames != 0 {
		t.Errorf("output in the echo window: %d echo frames and %d plain, want 1 and 0", echoes, *frames)
	}
	ts.sent.Store(time.Now().Add(-echoWindow - time.Millisecond).UnixNano())
	ts.onOutput("s", []byte("b"))
	if echoes != 1 || *frames != 1 {
		t.Errorf("output past it: %d echo frames and %d plain, want 1 and 1", echoes, *frames)
	}
}

// A program tracking the mouse (Claude Code turns it on) gets a report per
// pointer move; that is no one typing, and must not keep its output off the
// looping frame. A press, a release or a turn of the wheel is someone at the
// program, and its answer is drawn at once.
func TestAMouseReportOpensNoEchoWindowUnlessItIsAPressOrTheWheel(t *testing.T) {
	ts, th, frames := ridingTerminals(t)
	v := ts.byID["s"]
	v.tag = new(int)
	// Read-only: the writes stop short of the stream this test has none of.
	v.readOnly.Store(true)
	v.vt.Write([]byte("\x1b[?1003h\x1b[?1006h"))
	var router input.Router
	frame := func() {
		ops := new(op.Ops)
		gtx := layout.Context{Ops: ops, Source: router.Source(), Constraints: layout.Exact(image.Pt(800, 480))}
		ts.input(gtx, v, false)
		ts.scroll(gtx, v)
		area := clip.Rect{Max: image.Pt(800, 480)}.Push(ops)
		event.Op(ops, v.tag)
		area.Pop()
		router.Frame(ops)
	}
	frame()
	router.Queue(ptr(pointer.Move, at(3, 2), 0, 0), ptr(pointer.Move, at(4, 2), 0, 0))
	frame()
	if ts.sent.Load() != 0 {
		t.Fatal("a motion report opened the echo window")
	}
	spin(th, time.Now())
	ts.onOutput("s", []byte("a"))
	if *frames != 0 {
		t.Errorf("output after motion reports asked for %d frames, want to ride the loop", *frames)
	}
	for name, e := range map[string]pointer.Event{
		"press":   ptr(pointer.Press, at(4, 2), pointer.ButtonPrimary, 0),
		"release": ptr(pointer.Release, at(4, 2), 0, 0),
		"wheel":   {Kind: pointer.Scroll, Position: at(4, 2), Scroll: f32.Pt(0, wheelNotch)},
	} {
		ts.sent.Store(0)
		router.Queue(e)
		frame()
		if !ts.echoing(time.Now()) {
			t.Errorf("a %s report left the echo window shut", name)
		}
	}
}
