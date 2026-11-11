package ui

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
)

// beatWake draws a running service's beat at the instant now and reads the
// frame it asks for next.
func beatWake(th *Theme, now time.Time) (time.Time, bool) {
	var r input.Router
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Now: now, Source: r.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	th.PulseRing(gtx, f32.Pt(7, 7), 3, 3, th.TonePass)
	r.Frame(ops)
	return r.WakeupTime()
}

func TestServiceBeatPacing(t *testing.T) {
	th := &Theme{Palette: Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: Dark.Background}
	beat := time.UnixMilli(6 * 1000)
	inPulse, resting := beat.Add(100*time.Millisecond), beat.Add(1000*time.Millisecond)
	if at, ok := beatWake(th, inPulse); !ok || !at.Equal(inPulse.Add(42*time.Millisecond)) {
		t.Errorf("mid-pulse the beat asked for %v (asked %v), want the next frame 42ms on", at, ok)
	}
	if at, ok := beatWake(th, resting); !ok || !at.Equal(beat.Add(6*time.Second)) {
		t.Errorf("at rest the beat asked for %v (asked %v), want the next beat 6s on", at, ok)
	}
	th.Still = true
	if at, ok := beatWake(th, inPulse); !ok || !at.Equal(inPulse.Add(125*time.Millisecond)) {
		t.Errorf("on a still window mid-pulse the beat asked for %v (asked %v), want the slow frame 125ms on", at, ok)
	}
	if at, ok := beatWake(th, resting); !ok || !at.Equal(beat.Add(6*time.Second)) {
		t.Errorf("on a still window at rest the beat asked for %v (asked %v), want the next beat", at, ok)
	}
}
