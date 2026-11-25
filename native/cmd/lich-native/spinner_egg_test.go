package main

import (
	"testing"
	"time"
)

func neighbors(a, b int) bool {
	dx, dy := a%3-b%3, a/3-b/3
	return dx*dx+dy*dy == 1
}

func ghostRows(cells uint16) string {
	var l cellLevels
	for c := range cellCount {
		if cells&(1<<c) != 0 {
			l[c] = 1
		}
	}
	return cellRows(l)
}

// Each letter, once walked, is left whole in the faint cells.
func TestTheEggLeavesLICHFaintBehindTheHead(t *testing.T) {
	want := []string{"100/100/111", "010/010/010", "111/100/111", "101/111/101"}
	end := 0
	for i, w := range want {
		end += len(eggLetters[i].path)
		if got := ghostRows(eggSteps[end-1].ghost); got != w {
			t.Errorf("letter %d ends as %s, want %s", i, got, w)
		}
	}
	if last := eggSteps[len(eggSteps)-1]; last.ghost != 0 {
		t.Errorf("the egg's last step leaves %s faint, want nothing", ghostRows(last.ghost))
	}
}

// The egg moves like the snake: one neighboring cell at a time, into it from
// the lap's end and out of it to where either snake goes on.
func TestTheEggOnlyStepsToANeighboringCell(t *testing.T) {
	path := []int{spinnerPath[len(spinnerPath)-1]}
	for _, s := range eggSteps {
		path = append(path, s.head)
	}
	last := path[len(path)-1]
	path = append(path, spinnerPath[len(eggSteps)%len(spinnerPath)])
	for i := 1; i < len(path); i++ {
		if !neighbors(path[i-1], path[i]) {
			t.Errorf("step %d goes from %d to %d", i, path[i-1], path[i])
		}
	}
	if next := spinnerPath[(pathIndex(last)+1)%len(spinnerPath)]; !neighbors(last, next) {
		t.Errorf("the corner goes on from %d to %d", last, next)
	}
}

const (
	eggMinGap = 15 * time.Second
	eggMaxGap = 30 * time.Second
)

func checkEggGaps(t *testing.T, starts []time.Duration, span time.Duration) {
	t.Helper()
	if len(starts) < int(span/eggMaxGap) {
		t.Fatalf("%d eggs in %v, want one at least every %v", len(starts), span, eggMaxGap)
	}
	for i := 1; i < len(starts); i++ {
		if gap := starts[i] - starts[i-1]; gap < eggMinGap || gap > eggMaxGap {
			t.Errorf("eggs %d and %d are %v apart, want %v to %v", i-1, i, gap, eggMinGap, eggMaxGap)
		}
	}
}

func TestTheClockSnakesEggsAreFifteenToThirtySecondsApart(t *testing.T) {
	const span = 2 * time.Hour
	base := floorDiv(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC).UnixNano(), int64(spinnerStep))
	var starts []time.Duration
	for k := base; k < base+int64(span/spinnerStep); k++ {
		if e, ok := clockEgg(k); ok && e == 0 {
			if k%int64(len(spinnerPath)) != 0 {
				t.Fatalf("an egg starts on step %d, not on a lap's first", k)
			}
			starts = append(starts, time.Duration(k-base)*spinnerStep)
		}
	}
	checkEggGaps(t, starts, span)
}

// Read off the clock, an egg shows its head brightest and its letter faint.
func TestTheClockSpinnerShowsTheEggsLetter(t *testing.T) {
	k := floorDiv(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC).UnixNano(), int64(spinnerStep))
	for {
		if e, ok := clockEgg(k); ok && e == len(eggLetters[0].path)-1 {
			break
		}
		k++
	}
	if got := cellRows(clockLevels(k)); got != "100/100/123" {
		t.Errorf("the L's last step is %s, want 100/100/123", got)
	}
}

func TestACornerAtWorkWritesItsEggsFifteenToThirtySecondsApart(t *testing.T) {
	const span = 10 * time.Minute
	t0 := time.Date(2026, 11, 25, 0, 0, 0, 7, time.UTC)
	c := newCorner(cornerWork, t0)
	var starts []time.Duration
	prev := c.trail[0]
	for now := t0; now.Sub(t0) < span; now = now.Add(spinnerStep) {
		c.advance(now, cornerWork)
		if c.trail[0] != prev && prev >= 0 && !neighbors(prev, c.trail[0]) {
			t.Fatalf("at %v the head jumps from %d to %d", now.Sub(t0), prev, c.trail[0])
		}
		prev = c.trail[0]
		if c.egg == 0 {
			starts = append(starts, now.Sub(t0))
		}
	}
	checkEggGaps(t, starts, span)
}

// A corner told to wait mid-egg leaves the egg for the ring, letter and all.
func TestACornerLeavesItsEggForAStatusChange(t *testing.T) {
	t0 := time.Date(2026, 11, 25, 0, 0, 0, 7, time.UTC)
	c := newCorner(cornerWork, t0)
	now := t0
	for c.egg != len(eggLetters[0].path)-1 {
		now = now.Add(spinnerStep)
		c.advance(now, cornerWork)
	}
	c.advance(now.Add(spinnerStep), cornerWait)
	if c.egg != -1 || c.mode != cornerWait {
		t.Fatalf("after wait the corner is in mode %d with egg step %d, want waiting and no egg", c.mode, c.egg)
	}
	var want cellLevels
	want.lightTrail(c.trail[:])
	if got := c.levels(); got != want {
		t.Errorf("the ring shows %s, want only its trail, %s", cellRows(got), cellRows(want))
	}
}
