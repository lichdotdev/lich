package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
)

func views(list ...string) []sessionView {
	out := make([]sessionView, 0, len(list))
	for _, id := range list {
		out = append(out, sessionView{Session: lichclient.Session{ID: id}})
	}
	return out
}

func viewIDs(list []sessionView) string {
	out := ""
	for i, v := range list {
		if i > 0 {
			out += ","
		}
		out += v.ID
	}
	return out
}

func TestDepartedNamesWhatLeftWithItsSlot(t *testing.T) {
	gone := departed(views("a", "b", "c"), views("a", "c"))
	if len(gone) != 1 || gone[0].session.ID != "b" || gone[0].index != 1 {
		t.Fatalf("departed = %+v", gone)
	}
	if gone := departed(views("a", "b"), views("b", "a", "c")); len(gone) != 0 {
		t.Fatalf("a grown or reordered list departed %+v", gone)
	}
}

func TestWithClosingPutsCardsBackInSlotOrder(t *testing.T) {
	a, c := views("a")[0], views("c")[0]
	if got := withClosing(views("b"), []closingCard{{c, 2}, {a, 0}}); viewIDs(got) != "a,b,c" {
		t.Fatalf("shown = %s", viewIDs(got))
	}
	if got := withClosing(views("a"), []closingCard{{c, 2}}); viewIDs(got) != "a,c" {
		t.Fatalf("past the end = %s", viewIDs(got))
	}
}

func TestHoldDepartedKeepsSeparateClosesInDrawnOrder(t *testing.T) {
	first, _ := holdDeparted(nil, views("a", "b", "c", "d"), views("a", "c", "d"))
	second, _ := holdDeparted(first, views("a", "c", "d"), views("a", "d"))
	if got := withClosing(views("a", "d"), second); viewIDs(got) != "a,b,c,d" {
		t.Fatalf("shown = %s", viewIDs(got))
	}
	if _, changed := holdDeparted(second, views("a"), views("a", "c")); changed {
		t.Fatal("a list that only grew changed what is held")
	}
}

// Back while its card was still going, then closed again: one entry, in the
// slot it holds now, so its exit is timed from the second close.
func TestHoldDepartedHoldsACardClosedTwiceOnce(t *testing.T) {
	held, _ := holdDeparted(nil, views("a", "b", "c"), views("a", "c"))
	again, changed := holdDeparted(held, views("c", "a", "b"), views("c", "a"))
	if !changed || len(again) != 1 || again[0].session.ID != "b" || again[0].index != 2 {
		t.Fatalf("held = %+v (changed %v)", again, changed)
	}
}

func TestClosingCardsPlaysTheExitThenDrops(t *testing.T) {
	var c closingCards
	t0 := time.Unix(1000, 0)
	c.shown(views("a", "b", "c"), t0)
	shown, closing := c.shown(views("a", "c"), t0)
	if viewIDs(shown) != "a,b,c" || !closing["b"] || len(closing) != 1 {
		t.Fatalf("closing frame: %s %v", viewIDs(shown), closing)
	}
	if got := c.expiry(); !got.Equal(t0.Add(exitDuration)) {
		t.Fatalf("expiry = %v", got)
	}
	shown, _ = c.shown(views("a", "c"), t0.Add(exitDuration-time.Millisecond))
	if viewIDs(shown) != "a,b,c" {
		t.Fatalf("mid-exit: %s", viewIDs(shown))
	}
	shown, closing = c.shown(views("a", "c"), t0.Add(exitDuration))
	if viewIDs(shown) != "a,c" || len(closing) != 0 || !c.expiry().IsZero() {
		t.Fatalf("after exit: %s %v", viewIDs(shown), closing)
	}
}

// A second close restarts the clock for every held card.
func TestClosingCardsRestartsTheClockOnAnotherClose(t *testing.T) {
	var c closingCards
	t0 := time.Unix(1000, 0)
	c.shown(views("a", "b", "c"), t0)
	c.shown(views("a", "c"), t0)
	later := t0.Add(exitDuration / 2)
	c.shown(views("a"), later)
	shown, _ := c.shown(views("a"), t0.Add(exitDuration))
	if viewIDs(shown) != "a,b,c" {
		t.Fatalf("both cards should still be going: %s", viewIDs(shown))
	}
}

func TestClosingCardsLetsAReturningSessionBeLive(t *testing.T) {
	var c closingCards
	t0 := time.Unix(1000, 0)
	c.shown(views("a", "b"), t0)
	c.shown(views("a"), t0)
	shown, closing := c.shown(views("a", "b"), t0)
	if viewIDs(shown) != "a,b" || closing["b"] {
		t.Fatalf("returned: %s %v", viewIDs(shown), closing)
	}
}

func TestExitDurationIsPinned(t *testing.T) {
	// Pinned, not derived: the card's exit animation is timed from it.
	if exitDuration != 180*time.Millisecond {
		t.Fatalf("exitDuration = %v, want 180ms", exitDuration)
	}
}
