package main

import (
	"math"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-6 }

// A short card and a long one: the thumb keeps its length whichever of them
// fills the viewport.
func TestScrollSpanThumbHoldsAcrossUnevenCards(t *testing.T) {
	px := []int{60, 300, 2500}
	start, end, total := scrollSpan(px, 0, 0, 800)
	if total != 2860 || start != 0 || !near(end, 800.0/2860) {
		t.Fatalf("top: start %v end %v total %d", start, end, total)
	}
	start, end, _ = scrollSpan(px, 2, 1000, 800)
	if !near(start, 1360.0/2860) || !near(end, 2160.0/2860) {
		t.Fatalf("inside the long card: start %v end %v", start, end)
	}
}

func TestScrollSpanCountsUnmeasuredAsAverage(t *testing.T) {
	start, end, total := scrollSpan([]int{100, 0, 300}, 2, 0, 100)
	if total != 600 || !near(start, 300.0/600) || !near(end, 400.0/600) {
		t.Fatalf("start %v end %v total %d", start, end, total)
	}
}

func TestScrollSpanNothingMeasuredIsNotScrollable(t *testing.T) {
	if start, end, total := scrollSpan([]int{0, 0}, 0, 0, 500); start != 0 || end != 1 || total != 0 {
		t.Fatalf("start %v end %v total %d", start, end, total)
	}
}

func TestScrollSpanClampsPastTheEnd(t *testing.T) {
	if _, end, _ := scrollSpan([]int{100, 100}, 1, 50, 500); end != 1 {
		t.Fatalf("end %v", end)
	}
}

func TestItemHeightsResetOnlyWhenTheKeyOrCountMoves(t *testing.T) {
	var h itemHeights
	key := heightsKey{width: 600}
	h.reset(key, 2)
	h.px[0] = 40
	h.reset(key, 2)
	if h.px[0] != 40 {
		t.Fatal("same key and count dropped the heights")
	}
	h.reset(heightsKey{width: 700}, 2)
	if h.px[0] != 0 {
		t.Fatal("a new width kept the old heights")
	}
	h.px[0] = 40
	h.reset(heightsKey{width: 700}, 3)
	if len(h.px) != 3 || h.px[0] != 0 {
		t.Fatal("a new count kept the old heights")
	}
}
