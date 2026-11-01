package main

import (
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// beatWake draws a running service's beat at the instant now and reads the
// frame it asks for next.
func beatWake(th *ui.Theme, now time.Time) (time.Time, bool) {
	var r input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Now: now, Source: r.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	pulseRing(gtx, th, f32.Pt(7, 7), 3, 3, th.TonePass)
	r.Frame(ops)
	return r.WakeupTime()
}

func TestServiceBeatPacing(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	beat := time.UnixMilli(2400 * 1000)
	inPulse, resting := beat.Add(100*time.Millisecond), beat.Add(1000*time.Millisecond)
	if at, ok := beatWake(th, inPulse); !ok || !at.Equal(inPulse.Add(33*time.Millisecond)) {
		t.Errorf("mid-pulse the beat asked for %v (asked %v), want the next frame 33ms on", at, ok)
	}
	if at, ok := beatWake(th, resting); !ok || !at.Equal(beat.Add(2400*time.Millisecond)) {
		t.Errorf("at rest the beat asked for %v (asked %v), want the next beat", at, ok)
	}
	th.Still = true
	for _, now := range []time.Time{inPulse, resting} {
		if at, ok := beatWake(th, now); ok {
			t.Errorf("on a still window the beat asked for a frame at %v, want none", at)
		}
	}
}
