package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

// playCorner steps c toward want n times, a step at a time, and records its
// levels after each.
func playCorner(c *corner, want cornerMode, n int) []string {
	var got []string
	for range n {
		c.advance(c.at.Add(c.stepLen()), want)
		got = append(got, cellRows(c.levels()))
	}
	return got
}

func cornerAt(mode cornerMode, steps int) *corner {
	c := newCorner(mode, time.Unix(1000, 0))
	playCorner(c, mode, steps)
	return c
}

func TestAWaitingSessionCirclesTheBorder(t *testing.T) {
	c := cornerAt(cornerWait, 0)
	want := []string{"300/000/000", "230/000/000", "123/000/000", "012/003/000", "001/002/003", "000/001/032", "000/000/321", "000/300/210", "300/200/100"}
	assertSteps(t, playCorner(c, cornerWait, len(want)), want)
}

// The snake turns into the ring on the first border cell its head reaches,
// and carries on around it from there.
func TestWorkTurnsIntoTheRingWhereTheHeadMeetsTheBorder(t *testing.T) {
	c := cornerAt(cornerWork, 5) // the head on the center
	want := []string{"030/020/010", "023/010/000", "012/003/000", "001/002/003"}
	assertSteps(t, playCorner(c, cornerWait, len(want)), want)
	if c.mode != cornerWait || c.tone != cornerWait {
		t.Errorf("mode %d tone %d, want the ring in its color", c.mode, c.tone)
	}
}

// The ring hands the snake back on the cell the snake's path starts from.
func TestWaitTurnsBackIntoTheSnakeAtTheTopLeft(t *testing.T) {
	c := cornerAt(cornerWait, 3) // the head on the top right
	got := playCorner(c, cornerWork, 8)
	want := []string{"012/003/000", "001/002/003", "000/001/032", "000/000/321", "000/300/210", "300/200/100", "200/300/000", "100/200/300"}
	assertSteps(t, got, want)
	if c.mode != cornerWork {
		t.Errorf("mode %d, want the snake", c.mode)
	}
}

// A turn that ends fills the block from wherever the snake is: each cell it
// passes stays lit, and once all are, the block stands still.
func TestWorkFillsTheBlockAndStops(t *testing.T) {
	c := cornerAt(cornerWork, 6)
	got := playCorner(c, cornerDone, 20)
	if last := got[len(got)-1]; last != "333/333/333" {
		t.Fatalf("the corner ends at %s, want the whole block", last)
	}
	for i, step := range got {
		if i > 0 && lit(step) < lit(got[i-1]) {
			t.Errorf("step %d (%s) put out a cell of %s", i, step, got[i-1])
		}
	}
	if _, due := c.due(c.at.Add(cornerSettle).Add(toneBlend)); due {
		t.Error("a settled block still asks for frames")
	}
	if _, due := c.due(c.at); !due {
		t.Error("the block asks for no frame while it still glides")
	}
}

// A new turn on a finished corner starts the snake at its path's first cell,
// and every cell it passes leaves the block with its tail.
func TestANewTurnDrainsTheBlockIntoTheSnake(t *testing.T) {
	c := cornerAt(cornerDone, 0)
	got := playCorner(c, cornerWork, 12)
	want := []string{"333/333/333", "233/333/333", "133/233/333", "033/133/233", "033/033/123", "033/023/013", "023/013/003", "012/003/003", "001/002/003"}
	assertSteps(t, got[:len(want)], want)
	if c.mode != cornerWork || c.tone != cornerWork {
		t.Errorf("mode %d tone %d, want the snake in its color", c.mode, c.tone)
	}
}

// A finished session that goes straight to waiting drains first, then the
// snake turns into the ring.
func TestDoneToWaitingDrainsThenCircles(t *testing.T) {
	c := cornerAt(cornerDone, 0)
	playCorner(c, cornerWait, 20)
	if c.mode != cornerWait {
		t.Errorf("mode %d, want the ring", c.mode)
	}
}

func TestTheColorCrossesIntoTheNewStatusTone(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	c := cornerAt(cornerWork, 5)
	playCorner(c, cornerWait, 2)
	if got := c.color(th, c.toneAt); got != th.Primary {
		t.Errorf("as it turns the color is %v, want Primary %v", got, th.Primary)
	}
	if got := c.color(th, c.toneAt.Add(toneBlend)); got != th.ToneWait {
		t.Errorf("after the blend the color is %v, want ToneWait %v", got, th.ToneWait)
	}
}

// A corner not drawn for an hour, out of sight or under the pointer, catches
// up without replaying every step it missed.
func TestACornerSkipsAheadAfterALongGap(t *testing.T) {
	c := cornerAt(cornerWork, 0)
	later := c.at.Add(time.Hour)
	c.advance(later, cornerWork)
	if c.at.Before(later.Add(-spinnerStep)) {
		t.Errorf("the corner's step started at %v, want within a step of %v", c.at, later)
	}
}

func TestEachStatusPicksItsMode(t *testing.T) {
	cases := []struct {
		sv   sessionView
		want cornerMode
		ok   bool
	}{
		{sessionView{status: statusBusy}, cornerWork, true},
		{sessionView{status: statusCompacting}, cornerWork, true},
		{sessionView{status: statusWaiting}, cornerWait, true},
		{sessionView{status: statusDone, unread: true}, cornerDone, true},
		{sessionView{status: statusDone}, 0, false},
		{sessionView{status: statusNone}, 0, false},
	}
	for _, tc := range cases {
		if got, ok := cornerWant(&tc.sv); got != tc.want || ok != tc.ok {
			t.Errorf("status %q unread %v: %d, %v; want %d, %v", tc.sv.status, tc.sv.unread, got, ok, tc.want, tc.ok)
		}
	}
}

func lit(rows string) int {
	n := 0
	for _, r := range rows {
		if r != '0' && r != '/' {
			n++
		}
	}
	return n
}

func assertSteps(t *testing.T, got, want []string) {
	t.Helper()
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("steps %v, want %v", got, want)
		}
	}
}
