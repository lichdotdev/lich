package ui

import (
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/op/clip"
)

// loopWake draws w once at now and reads the frame it asks for next.
func loopWake(w func(C) D) (time.Time, bool) {
	var r input.Router
	tabsFrame(&r, time.Unix(100, 0), w)
	return r.WakeupTime()
}

// Every frame of a looping animation repaints the whole window: one asking
// at the display's rate held a laptop GPU at 60%, so the next frame comes
// 33ms later, never at once.
func TestLoopingAnimationsRedrawAt30fps(t *testing.T) {
	th := testTheme()
	want := time.Unix(100, 0).Add(33 * time.Millisecond)
	for name, w := range map[string]func(C) D{
		"Spinner":  func(gtx C) D { return th.Spinner(gtx, 16, th.Foreground) },
		"BusyRing": func(gtx C) D { return th.BusyRing(gtx, 16, th.Foreground, th.Border) },
	} {
		if at, ok := loopWake(w); !ok || !at.Equal(want) {
			t.Errorf("%s asked for its next frame at %v (asked %v), want %v", name, at, ok, want)
		}
	}
}

func TestStillThemeHoldsTheFrame(t *testing.T) {
	th := testTheme()
	th.Still = true
	for name, w := range map[string]func(C) D{
		"Spinner":  func(gtx C) D { return th.Spinner(gtx, 16, th.Foreground) },
		"BusyRing": func(gtx C) D { return th.BusyRing(gtx, 16, th.Foreground, th.Border) },
	} {
		if at, ok := loopWake(w); ok {
			t.Errorf("%s on a still window asked for a frame at %v, want none", name, at)
		}
	}
}

func TestStrokeArcPathsAreRecordedOnce(t *testing.T) {
	a := arcPaths{paths: map[arcKey]clip.PathSpec{}}
	k := arcKey{center: f32.Pt(5, 5), radius: 4, sweep: 1}
	if a.get(k) != a.get(k) {
		t.Error("the same arc came back as a new recording")
	}
	if a.get(k) == a.get(arcKey{center: f32.Pt(5, 5), radius: 4, sweep: 2}) {
		t.Error("another arc reused the first's path")
	}
	first := a.get(k)
	for i := range arcPathsMax {
		a.get(arcKey{radius: float32(i), sweep: 1})
	}
	if len(a.paths) > arcPathsMax || a.get(k) == first {
		t.Errorf("after %d arcs the record holds %d and still hands out the old recording", arcPathsMax, len(a.paths))
	}
}
