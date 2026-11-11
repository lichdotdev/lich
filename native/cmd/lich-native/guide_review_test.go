package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestReviewGuideNeedsAnAgentWithChanges(t *testing.T) {
	session := func(kind string, files int) sessionView {
		sv := sessionView{git: backend.DiffStats{Files: files}}
		sv.Kind = kind
		return sv
	}
	cases := []struct {
		name string
		sv   sessionView
		want bool
	}{
		{"an agent with changes", session("claude", 2), true},
		{"an agent with nothing changed", session("claude", 0), false},
		{"a terminal, which refuses the batch", session("shell", 2), false},
	}
	for _, c := range cases {
		if got := canReview(c.sv); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReviewGuideWaitsOnAnyCardsComposer(t *testing.T) {
	p := &reviewPanel{cards: map[string]*fileCard{"a.go": {}, "b.go": {}}}
	if p.composing() {
		t.Fatal("composing with every composer closed")
	}
	p.cards["b.go"].composer.open("b.go", "3-4")
	if !p.composing() {
		t.Error("not composing with b.go's composer open")
	}
}

func TestReviewPanelWithoutATourStillLaysOutItsSpots(t *testing.T) {
	var drawn bool
	(&reviewPanel{}).spot(C{}, tourReviewComposer, func(C) D { drawn = true; return D{} })
	if !drawn {
		t.Error("the Pulls screen's panel, which has no tour, skipped the widget")
	}
}
