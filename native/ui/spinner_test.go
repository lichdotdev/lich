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

// A frame drawn between beats, for input or output, already shows the ring
// where the beat would: it takes the beat's place, and the next beat comes
// one LoopFrame after the one it replaced. Pushing the beat a LoopFrame past
// every such frame drifted it off its phase; keeping the beat as well drew
// both, 49fps against 40 with the pointer moving over a terminal.
func TestLoopKeepsTheBeatsPhase(t *testing.T) {
	th := testTheme()
	th.Beat = &LoopBeat{}
	start := time.Unix(100, 0)
	// Two rings a frame, as a sidebar with two busy sessions draws.
	at := func(now time.Time) time.Time {
		var r input.Router
		tabsFrame(&r, now, func(gtx C) D {
			th.BusyRing(gtx, 16, th.Foreground, th.Border)
			return th.BusyRing(gtx, 16, th.Foreground, th.Border)
		})
		wake, _ := r.WakeupTime()
		return wake
	}
	beat := start.Add(LoopFrame)
	if got := at(start); !got.Equal(beat) {
		t.Fatalf("first frame asked for %v, want %v", got, beat)
	}
	next := beat.Add(LoopFrame)
	if got := at(start.Add(10 * time.Millisecond)); !got.Equal(next) {
		t.Errorf("a frame 10ms before the beat asked for %v, want it to stand in and the next beat at %v", got, next)
	}
	if got := at(start.Add(30 * time.Millisecond)); !got.Equal(next) {
		t.Errorf("a second frame before that beat asked for %v, want the beat kept at %v", got, next)
	}
	if got := at(next.Add(3 * time.Millisecond)); !got.Equal(next.Add(LoopFrame)) {
		t.Errorf("the beat drawn 3ms late asked for %v, want one LoopFrame after the beat it was, %v", got, next.Add(LoopFrame))
	}
	if th.Beat.Pending(next.Add(LoopFrame)) {
		t.Error("the second ring of the beat's frame moved the beat on once more")
	}
	late := next.Add(10 * LoopFrame)
	if got := at(late); !got.Equal(late.Add(LoopFrame)) {
		t.Errorf("a frame long after the last beat asked for %v, want a fresh one LoopFrame out, %v", got, late.Add(LoopFrame))
	}
	// A still window's beat, StillFrame out, when the window takes focus.
	focused := late.Add(LoopFrame / 2)
	th.Beat.due.Store(focused.Add(StillFrame).UnixNano())
	if got := at(focused); !got.Equal(focused.Add(LoopFrame)) {
		t.Errorf("a still beat further out than LoopFrame asked for %v, want it brought in to %v", got, focused.Add(LoopFrame))
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
