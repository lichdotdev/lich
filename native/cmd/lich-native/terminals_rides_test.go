package main

import (
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
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
