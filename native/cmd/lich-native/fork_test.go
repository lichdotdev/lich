package main

import (
	"slices"
	"testing"
)

func TestForkLabelTakesTheNextFreeNumberOfItsBase(t *testing.T) {
	cases := []struct {
		parent string
		taken  []string
		want   string
	}{
		{"auth-refactor", []string{"auth-refactor"}, "auth-refactor 2"},
		{"auth-refactor 2", []string{"auth-refactor", "auth-refactor 2"}, "auth-refactor 3"},
		{"auth-refactor", []string{"auth-refactor", "auth-refactor 2", "auth-refactor 3"}, "auth-refactor 4"},
		{"auth-refactor 2", []string{"auth-refactor 2", "auth-refactor 4"}, "auth-refactor 3"},
		{"refresh queue", []string{"refresh queue", "auth-refactor 2"}, "refresh queue 2"},
		{"Claude 3", []string{"Claude 3"}, "Claude 4"},
		{"v2", []string{"v2"}, "v2 2"},
	}
	for _, c := range cases {
		if got := forkLabel(c.parent, c.taken); got != c.want {
			t.Errorf("forkLabel(%q, %v) = %q, want %q", c.parent, c.taken, got, c.want)
		}
	}
}

func TestOnlyTheThreeProvidersThatBranchAConversationFork(t *testing.T) {
	for kind, want := range map[string]bool{
		"claude": true, "codex": true, "opencode": true,
		"omp": false, "crush": false, "cursor": false, "kiro": false, "antigravity": false, "shell": false,
	} {
		if got := canFork(kind); got != want {
			t.Errorf("canFork(%q) = %v, want %v", kind, got, want)
		}
	}
}

func TestPlacedAfterPutsTheForkUnderItsParent(t *testing.T) {
	cases := []struct {
		ids        []string
		id, parent string
		want       []string
	}{
		{[]string{"a", "p", "b", "f"}, "f", "p", []string{"a", "p", "f", "b"}},
		{[]string{"p", "a"}, "f", "p", []string{"p", "f", "a"}},
		{[]string{"a", "b"}, "f", "gone", []string{"a", "b", "f"}},
	}
	for _, c := range cases {
		if got := placedAfter(c.ids, c.id, c.parent); !slices.Equal(got, c.want) {
			t.Errorf("placedAfter(%v, %q, %q) = %v, want %v", c.ids, c.id, c.parent, got, c.want)
		}
	}
}

func TestTheSidebarKeepsACardsForkRequests(t *testing.T) {
	var frame sidebarAction
	frame.takeCard(sidebarAction{forkHere: "s1"})
	frame.takeCard(sidebarAction{forkWorktree: "s2"})
	if frame.forkHere != "s1" || frame.forkWorktree != "s2" {
		t.Fatalf("frame action %+v, want both fork requests kept", frame)
	}
}
