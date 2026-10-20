package main

import "testing"

func cardsOf(ids ...string) []hotkeyCard {
	cards := make([]hotkeyCard, len(ids))
	for i, id := range ids {
		cards[i] = hotkeyCard{id: id, shown: true}
	}
	return cards
}

func all(string) bool { return true }

func TestNextShownCardStepsForwardAndBackward(t *testing.T) {
	cards := cardsOf("s1", "s2", "s3")
	if got := nextShownCard(cards, "s1", 1, all); got != "s2" {
		t.Errorf("forward: %q", got)
	}
	if got := nextShownCard(cards, "s2", -1, all); got != "s1" {
		t.Errorf("backward: %q", got)
	}
}

func TestNextShownCardWrapsAtBothEnds(t *testing.T) {
	cards := cardsOf("s1", "s2", "s3")
	if got := nextShownCard(cards, "s3", 1, all); got != "s1" {
		t.Errorf("forward: %q", got)
	}
	if got := nextShownCard(cards, "s1", -1, all); got != "s3" {
		t.Errorf("backward: %q", got)
	}
}

func TestNextShownCardHasNowhereToGoWithZeroOrOneSession(t *testing.T) {
	for name, c := range map[string][]hotkeyCard{"none": nil, "one": cardsOf("s1")} {
		if got := nextShownCard(c, "s1", 1, all); got != "" {
			t.Errorf("%s forward: %q", name, got)
		}
		if got := nextShownCard(c, "s1", -1, all); got != "" {
			t.Errorf("%s backward: %q", name, got)
		}
	}
}

// The active session can be gone (closed under a stale render), so the press
// still lands on the end it steps in from instead of doing nothing.
func TestNextShownCardFallsBackToAnEndOfTheListForAnUnknownSession(t *testing.T) {
	cards := cardsOf("s1", "s2", "s3")
	if got := nextShownCard(cards, "gone", 1, all); got != "s1" {
		t.Errorf("forward: %q", got)
	}
	if got := nextShownCard(cards, "gone", -1, all); got != "s3" {
		t.Errorf("backward: %q", got)
	}
}

// Folded blocks hide cards the walk used to land on: the press moved focus to
// a session the user could not see.
func TestNextShownCardStepsOverTheCardsOfAFoldedBlock(t *testing.T) {
	cards := []hotkeyCard{{"s1", true}, {"wt1", false}, {"s2", true}}
	if got := nextShownCard(cards, "s1", 1, all); got != "s2" {
		t.Errorf("forward: %q", got)
	}
	if got := nextShownCard(cards, "s2", -1, all); got != "s1" {
		t.Errorf("backward: %q", got)
	}
}

func waitingOn(ids ...string) func(string) bool {
	return func(id string) bool {
		for _, w := range ids {
			if w == id {
				return true
			}
		}
		return false
	}
}

func TestNextWaitingSkipsSessionsThatAreNotWaiting(t *testing.T) {
	if got := nextShownCard(cardsOf("s1", "s2", "s3", "s4"), "s1", 1, waitingOn("s3")); got != "s3" {
		t.Errorf("got %q", got)
	}
}

func TestNextWaitingTakesTheNearestAfterTheActiveOne(t *testing.T) {
	cards := cardsOf("s1", "s2", "s3", "s4")
	if got := nextShownCard(cards, "s1", 1, waitingOn("s2", "s4")); got != "s2" {
		t.Errorf("from s1: %q", got)
	}
	if got := nextShownCard(cards, "s2", 1, waitingOn("s2", "s4")); got != "s4" {
		t.Errorf("from s2: %q", got)
	}
}

func TestNextWaitingWrapsPastTheEndOfTheList(t *testing.T) {
	if got := nextShownCard(cardsOf("s1", "s2", "s3", "s4"), "s3", 1, waitingOn("s1")); got != "s1" {
		t.Errorf("got %q", got)
	}
}

func TestNextWaitingAnswersNothingWhenNoSessionOrOnlyTheActiveOneWaits(t *testing.T) {
	cards := cardsOf("s1", "s2", "s3")
	if got := nextShownCard(cards, "s1", 1, waitingOn()); got != "" {
		t.Errorf("none waiting: %q", got)
	}
	if got := nextShownCard(cards, "s2", 1, waitingOn("s2")); got != "" {
		t.Errorf("only the active one: %q", got)
	}
}

func TestNextWaitingSearchesTheWholeListForAnUnknownSession(t *testing.T) {
	if got := nextShownCard(cardsOf("s1", "s2", "s3"), "gone", 1, waitingOn("s3")); got != "s3" {
		t.Errorf("got %q", got)
	}
}

// Home comes first, the way the strip draws it.
func TestNeighborProject(t *testing.T) {
	strip := []string{"home", "a", "b"}
	step := func(ids []string, active string, by int) string {
		id, _ := neighborProject(ids, active, by)
		return id
	}
	if got := step(strip, "home", 1); got != "a" {
		t.Errorf("home +1: %q", got)
	}
	if got := step(strip, "a", 1); got != "b" {
		t.Errorf("a +1: %q", got)
	}
	if got := step(strip, "b", 1); got != "home" {
		t.Errorf("b +1 wraps: %q", got)
	}
	if got := step(strip, "home", -1); got != "b" {
		t.Errorf("home -1 wraps: %q", got)
	}
	if _, ok := neighborProject([]string{"home"}, "home", 1); ok {
		t.Error("a single tab has somewhere to go")
	}
	if _, ok := neighborProject(nil, "", 1); ok {
		t.Error("no tab has somewhere to go")
	}
	if got := step(strip, "", 1); got != "home" {
		t.Errorf("nothing active +1: %q", got)
	}
	if got := step(strip, "", -1); got != "b" {
		t.Errorf("nothing active -1: %q", got)
	}
	stored := []string{"a", "b", "home"}
	if got := step(stored, "b", 1); got != "home" {
		t.Errorf("without Home, b +1: %q", got)
	}
	if got := step(stored, "a", -1); got != "home" {
		t.Errorf("without Home, a -1: %q", got)
	}
}
