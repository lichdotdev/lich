package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/i18n"
)

func TestFirstTourOpensOnlyAfterAFirstRunSetup(t *testing.T) {
	cases := []struct {
		name                  string
		setupShown, setupDone bool
		seen                  string
		want                  bool
	}{
		{"fresh install, setup closed", true, true, "", true},
		{"setup still open", true, false, "", false},
		{"no setup this launch, an existing install", false, false, "", false},
		{"seen before", true, true, "1", false},
	}
	for _, c := range cases {
		if got := decideFirstTour(c.setupShown, c.setupDone, c.seen); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The step keys live in a table, out of the literal-key scan's sight, so
// they are checked here.
func TestEveryBasicsStepNamesKeysInTheCatalog(t *testing.T) {
	check := func(key string) {
		if key != "" && i18n.T(key) == key {
			t.Errorf("%q is not in the catalog", key)
		}
	}
	for i, s := range basicsTour {
		for _, w := range []*tourText{&s.text, s.shell} {
			if w == nil {
				continue
			}
			check(w.title)
			check(w.body)
			check(w.hint)
		}
		if (s.until != nil) != (s.text.hint != "") {
			t.Errorf("step %d: a hint names the action a waiting step expects, and only one", i)
		}
	}
}

func TestBasicsWaitsOnlyForOpeningAProjectAndASession(t *testing.T) {
	var waiting []string
	for _, s := range basicsTour {
		if s.until != nil {
			waiting = append(waiting, s.anchor)
		}
	}
	if len(waiting) != 2 || waiting[0] != tourOpen || waiting[1] != tourNewEmpty {
		t.Fatalf("waiting steps = %v, want the open-project and new-session buttons", waiting)
	}
	if last := basicsTour[len(basicsTour)-1].anchor; last != tourMenu {
		t.Fatalf("the tour ends on %q, want the menu that holds it", last)
	}
}
