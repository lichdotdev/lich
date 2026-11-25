package main

import (
	"math/rand/v2"
	"time"
)

// The working snake now and then writes lich: the same head and tail walk
// L, I, C and H, the part of the letter already walked staying faint behind
// them, and the snake goes back to its lap. It is meant to be caught, not
// announced, so it looks like the snake taking another way round.

// eggLetter is a stretch of the egg's path. seed is a cell the letter has
// before the head walks it, the one the last letter ended on; a stretch with
// no letter leaves nothing faint.
type eggLetter struct {
	path   []int
	seed   []int
	letter bool
}

// Every move is to a neighboring cell, as on spinnerPath: the egg starts on
// the lap's first cell, and its last cell, 7, neighbors where the lap goes on
// from (spinnerPath[len(eggSteps)%len(spinnerPath)], 8, for the clock's snake).
var eggLetters = [...]eggLetter{
	{path: []int{0, 3, 6, 7, 8}, letter: true},
	{path: []int{7, 4, 1}, letter: true},
	{path: []int{2, 1, 0, 3, 6, 7, 8}, letter: true},
	{path: []int{5, 2, 5, 4, 3, 0, 3, 6}, seed: []int{8}, letter: true},
	{path: []int{7}},
}

// eggStep is a step of the egg: where its head is and the cells left faint.
type eggStep struct {
	head  int
	ghost uint16
}

var eggSteps = walkEgg()

func walkEgg() []eggStep {
	var steps []eggStep
	for _, l := range eggLetters {
		var ghost uint16
		for _, c := range l.seed {
			ghost |= 1 << c
		}
		for _, c := range l.path {
			if l.letter {
				ghost |= 1 << c
			}
			steps = append(steps, eggStep{head: c, ghost: ghost})
		}
	}
	return steps
}

const (
	// eggGhost is the level the walked part of a letter keeps.
	eggGhost uint8 = 1
	// eggEvery is the slot each egg falls in, at most eggSpread into it. An
	// egg waits for the snake's next lap, up to a lap after it falls, so two
	// eggs are 15 to 30 seconds apart.
	eggEvery  = 22500 * time.Millisecond
	eggSpread = 6 * time.Second
	eggLap    = time.Duration(len(spinnerPath)) * spinnerStep
)

// eggAt is when slot n's egg falls, in nanoseconds since the epoch: the same
// for every spinner in the window, and for every window, with no state.
func eggAt(n int64) int64 {
	off := rand.NewPCG(uint64(n), 0).Uint64() % uint64(eggSpread)
	return n*int64(eggEvery) + int64(off)
}

// eggStarts reports whether a lap starting at the snake's step at begins an
// egg: the first lap to start once its slot's egg has fallen.
func eggStarts(at time.Time) bool {
	t := at.UnixNano()
	e := eggAt(floorDiv(t, int64(eggEvery)))
	return e <= t && t-int64(eggLap) < e
}

// clockEgg is the egg's step at the clock's step k, false outside an egg.
// The clock's laps start on the steps spinnerPath's length divides.
func clockEgg(k int64) (int, bool) {
	t := k * int64(spinnerStep)
	n := int64(len(spinnerPath))
	e := eggAt(floorDiv(t, int64(eggEvery)))
	step := -floorDiv(-e, int64(spinnerStep))
	start := -floorDiv(-step, n) * n
	if k < start || k >= start+int64(len(eggSteps)) {
		return 0, false
	}
	return int(k - start), true
}

// clockHead is the head's cell at the clock's step k.
func clockHead(k int64) int {
	if e, ok := clockEgg(k); ok {
		return eggSteps[e].head
	}
	n := int64(len(spinnerPath))
	return spinnerPath[(k%n+n)%n]
}

// clockLevels is the clock's snake at step k, an egg's letter included.
func clockLevels(k int64) cellLevels {
	var l cellLevels
	if e, ok := clockEgg(k); ok {
		l.lightGhost(eggSteps[e].ghost)
	}
	var trail [spinnerTrail]int
	for j := range trail {
		trail[j] = clockHead(k - int64(j))
	}
	l.lightTrail(trail[:])
	return l
}

func (l *cellLevels) lightGhost(cells uint16) {
	for c := range cellCount {
		if cells&(1<<c) != 0 {
			l[c] = max(l[c], eggGhost)
		}
	}
}
