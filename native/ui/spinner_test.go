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
// 42ms later, never at once.
func TestLoopingAnimationsRedrawAt24fps(t *testing.T) {
	th := testTheme()
	want := time.Unix(100, 0).Add(42 * time.Millisecond)
	for name, w := range map[string]func(C) D{
		"Spinner":  func(gtx C) D { return th.Spinner(gtx, 16, th.Foreground) },
		"BusyRing": func(gtx C) D { return th.BusyRing(gtx, 16, th.Foreground, th.Border) },
	} {
		if at, ok := loopWake(w); !ok || !at.Equal(want) {
			t.Errorf("%s asked for its next frame at %v (asked %v), want %v", name, at, ok, want)
		}
	}
}

func TestLoopRecordsTheFrameItAskedFor(t *testing.T) {
	th := testTheme()
	th.Beat = &LoopBeat{}
	now := time.Unix(100, 0)
	if th.Beat.Pending(now) {
		t.Fatal("a beat nobody asked for is pending")
	}
	loopWake(func(gtx C) D { return th.Spinner(gtx, 16, th.Foreground) })
	if !th.Beat.Pending(now.Add(41 * time.Millisecond)) {
		t.Error("41ms after the spinner drew, its frame is still to come")
	}
	if th.Beat.Pending(now.Add(42 * time.Millisecond)) {
		t.Error("42ms after the spinner drew, its frame is due")
	}
	th.Still = true
	th.Beat = &LoopBeat{}
	loopWake(func(gtx C) D { return th.Spinner(gtx, 16, th.Foreground) })
	if !th.Beat.Pending(now.Add(124 * time.Millisecond)) {
		t.Error("a still window's spinner did not record the slow frame it asked for")
	}
}

// A window without focus is still seen: its animations go on at 8fps.
func TestStillThemeSlowsTheLoop(t *testing.T) {
	th := testTheme()
	th.Still = true
	want := time.Unix(100, 0).Add(125 * time.Millisecond)
	for name, w := range map[string]func(C) D{
		"Spinner":  func(gtx C) D { return th.Spinner(gtx, 16, th.Foreground) },
		"BusyRing": func(gtx C) D { return th.BusyRing(gtx, 16, th.Foreground, th.Border) },
	} {
		if at, ok := loopWake(w); !ok || !at.Equal(want) {
			t.Errorf("%s on a still window asked for its next frame at %v (asked %v), want %v", name, at, ok, want)
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
