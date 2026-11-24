package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

// playDots steps d toward want n times and records its levels after each.
func playDots(d *tabDots, want cornerMode, n int) []string {
	var got []string
	for range n {
		d.advance(d.at.Add(d.stepLen()), want)
		l := d.levels()
		got = append(got, fmt.Sprintf("%d%d%d", l[0], l[1], l[2]))
	}
	return got
}

func dotsAt(mode cornerMode, steps int) *tabDots {
	d := newTabDots(mode, time.Unix(1000, 0))
	playDots(d, mode, steps)
	return d
}

func TestAWorkingTabRunsItsDotsLeftToRight(t *testing.T) {
	want := []string{"300", "230", "123", "012", "001", "300"}
	assertSteps(t, playDots(dotsAt(cornerWork, 0), cornerWork, len(want)), want)
}

func TestAWaitingTabGoesThereAndBack(t *testing.T) {
	want := []string{"300", "230", "123", "032", "321", "230"}
	assertSteps(t, playDots(dotsAt(cornerWait, 0), cornerWait, len(want)), want)
}

// The run turns back as the pong where its head heads right on a dot.
func TestWorkTurnsIntoThePongHeadingRight(t *testing.T) {
	d := dotsAt(cornerWork, 3) // the head on the right end
	want := []string{"012", "001", "300", "230", "123", "032"}
	assertSteps(t, playDots(d, cornerWait, len(want)), want)
	if d.mode != cornerWait || d.tone != cornerWait {
		t.Errorf("mode %d tone %d, want the pong in its color", d.mode, d.tone)
	}
}

// The pong hands the run back on the right end, so the head leaves through
// the edge instead of jumping back to the left.
func TestWaitLeavesThroughTheRightEdge(t *testing.T) {
	d := dotsAt(cornerWait, 2) // the head on the middle, heading right
	want := []string{"123", "012", "001", "300"}
	assertSteps(t, playDots(d, cornerWork, len(want)), want)
	if d.mode != cornerWork {
		t.Errorf("mode %d, want the run", d.mode)
	}
}

// A turn that ends lights each dot the head passes, from the head on, then
// stands still and stops asking for frames.
func TestWorkFillsTheDotsAndStops(t *testing.T) {
	d := dotsAt(cornerWork, 3)
	want := []string{"012", "002", "302", "333"}
	assertSteps(t, playDots(d, cornerDone, len(want)), want)
	if _, due := d.due(d.at.Add(cornerSettle).Add(toneBlend)); due {
		t.Error("finished dots still ask for frames")
	}
}

func TestANewTurnDrainsTheDotsFromTheLeft(t *testing.T) {
	d := dotsAt(cornerDone, 0)
	want := []string{"333", "233", "123", "012"}
	assertSteps(t, playDots(d, cornerWork, len(want)), want)
	if d.mode != cornerWork || d.tone != cornerWork {
		t.Errorf("mode %d tone %d, want the run in its color", d.mode, d.tone)
	}
}

func TestTheTabsColorCrossesIntoTheNewStatus(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	d := dotsAt(cornerWork, 1)
	playDots(d, cornerWait, 1)
	if got := blendTone(th, d.toneFrom, d.tone, 0); got != th.Primary {
		t.Errorf("as it turns the color is %v, want Primary %v", got, th.Primary)
	}
	if got := blendTone(th, d.toneFrom, d.tone, toneBlend); got != th.ToneWait {
		t.Errorf("after the blend the color is %v, want ToneWait %v", got, th.ToneWait)
	}
}

func TestEachTabStatusPicksItsMode(t *testing.T) {
	cases := map[string]struct {
		want cornerMode
		ok   bool
	}{
		statusBusy:       {cornerWork, true},
		statusCompacting: {cornerWork, true},
		statusWaiting:    {cornerWait, true},
		statusDone:       {cornerDone, true},
		statusNone:       {0, false},
	}
	for status, tc := range cases {
		if got, ok := tabWant(status); got != tc.want || ok != tc.ok {
			t.Errorf("status %q: %d, %v; want %d, %v", status, got, ok, tc.want, tc.ok)
		}
	}
}
