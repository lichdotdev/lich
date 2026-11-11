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

// The guides' keys live in tables, out of the literal-key scan's sight, so
// they are checked here, for every guide registered.
func TestEveryGuideNamesKeysInTheCatalog(t *testing.T) {
	check := func(g *guide, key string) {
		if key != "" && i18n.T(key) == key {
			t.Errorf("guide %s: %q is not in the catalog", g.id, key)
		}
	}
	for _, g := range guides {
		check(g, g.label)
		check(g, g.unavailable)
		if (g.ready != nil) != (g.unavailable != "") {
			t.Errorf("guide %s: a guide that can be unavailable says why, and only then", g.id)
		}
		for i, s := range g.steps {
			for _, w := range []*tourText{&s.text, s.shell} {
				if w == nil {
					continue
				}
				check(g, w.title)
				check(g, w.body)
				check(g, w.hint)
			}
			if (s.until != nil) != (s.text.hint != "") {
				t.Errorf("guide %s, step %d: a hint names the action a waiting step expects, and only one", g.id, i)
			}
			if s.layer > 0 && (s.reopen >= i || g.steps[s.reopen].layer >= s.layer) {
				t.Errorf("guide %s, step %d: a step under a menu or dialog goes back to an earlier step that opens it", g.id, i)
			}
			if s.anchor == "" && s.until != nil {
				t.Errorf("guide %s, step %d: a step that waits for an action points at where to do it", g.id, i)
			}
		}
	}
}

func TestGuidesAreUniqueAndBasicsComesFirst(t *testing.T) {
	ids, orders := map[string]bool{}, map[int]bool{}
	for _, g := range guides {
		if ids[g.id] || orders[g.order] {
			t.Errorf("guide %s: its id or order is taken", g.id)
		}
		ids[g.id], orders[g.order] = true, true
		if len(g.steps) == 0 {
			t.Errorf("guide %s has no steps", g.id)
		}
	}
	if len(guides) == 0 || guides[0].id != basicsID {
		t.Fatal("the Guides menu does not open with the basics")
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

// The handoff has no button: its one waiting step is the agent's terminal,
// where the person asks for it.
// The handoff depends on the agent having lich's tools and picking them, so
// no step may hold the tour until it happens.
func TestHandoffNeverWaitsOnTheAgent(t *testing.T) {
	for i, s := range handoffTour {
		if s.until != nil {
			t.Errorf("step %d waits; the handoff guide must never hold the tour", i)
		}
	}
	if handoffTour[0].anchor != tourStage {
		t.Fatal("the guide does not open on the terminal, where the agent is asked")
	}
}
