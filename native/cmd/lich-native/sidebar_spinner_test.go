package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// cellRows writes levels as three rows of three, top first.
func cellRows(l cellLevels) string {
	var b strings.Builder
	for c, v := range l {
		if c > 0 && c%3 == 0 {
			b.WriteByte('/')
		}
		fmt.Fprint(&b, v)
	}
	return b.String()
}

// The head goes down the left column, up the middle, down the right, then
// back right to left along the bottom, left to right along the middle and
// right to left along the top, each cell it leaves fading behind it.
func TestTheSnakesHeadWalksItsPathWithAFadingTail(t *testing.T) {
	want := []string{
		"321/000/000", "210/300/000", "100/200/300", "000/100/230",
		"000/030/120", "030/020/010", "023/010/000", "012/003/000",
		"001/002/003", "000/001/032", "000/000/321", "000/300/210",
		"000/230/100", "000/123/000", "003/012/000", "032/001/000",
	}
	for k, w := range want {
		if got := cellRows(snakeLevels(int64(k))); got != w {
			t.Errorf("step %d is %s, want %s", k, got, w)
		}
	}
	if snakeLevels(-1) != snakeLevels(int64(len(spinnerPath)-1)) {
		t.Error("a step before the first is not the path's last")
	}
}

func TestACellGlidesTowardItsLevelWithoutPassingIt(t *testing.T) {
	from := cellLevels{}.faded()
	to := cellLevels{spinnerTrail}
	if got := glide(from, to, 0)[0]; got != cellFade[0] {
		t.Errorf("at once the cell is %v, want where it was, %v", got, cellFade[0])
	}
	half := glide(from, to, cellGlide)[0]
	if half <= cellFade[0] || half >= cellFade[spinnerTrail] {
		t.Errorf("after one glide the cell is %v, want between %v and %v", half, cellFade[0], cellFade[spinnerTrail])
	}
	if got := glide(from, to, time.Second)[0]; cellFade[spinnerTrail]-got > 0.001 {
		t.Errorf("after a second the cell is %v, want its level, %v", got, cellFade[spinnerTrail])
	}
}

func TestTheClockSpinnerIsDueWithinAGlideFrameAtAnyTime(t *testing.T) {
	for _, now := range []time.Time{{}, time.Unix(0, 0), time.Unix(0, int64(spinnerStep)*17+1)} {
		_, next := spinnerAt(now)
		if wait := next.Sub(now); wait <= 0 || wait > glideFrame {
			t.Errorf("at %v the next frame is %v away, want within %v", now, wait, glideFrame)
		}
	}
}

// Read off the clock, the spinner shows the head of its step, past the
// tail's brightest level by the step's end.
func TestTheClockSpinnerShowsTheStepsHead(t *testing.T) {
	k := int64(21)
	end := time.Unix(0, (k+1)*int64(spinnerStep)-1)
	a, _ := spinnerAt(end)
	head := spinnerPath[k%int64(len(spinnerPath))]
	if a[head] <= cellFade[spinnerTrail-1] {
		t.Errorf("the head, cell %d, is %v at the step's end, want past the tail's %v", head, a[head], cellFade[spinnerTrail-1])
	}
	for c, v := range a {
		if c != head && v >= a[head] {
			t.Errorf("cell %d is %v, as bright as the head", c, v)
		}
	}
}
