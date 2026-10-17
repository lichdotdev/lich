package main

import (
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
)

const (
	filterProject  = "/home/u/code/lich"
	filterWorktree = "/home/u/.local/share/lich/worktrees/lich/feat/auth-relay"
)

var filterFixture = []lichclient.Session{
	{ID: "s1", Label: "Relay inbox nudge", Pinned: true},
	{ID: "s2", Label: "Patch notes", Pinned: true},
	{ID: "s3", Label: "Relay ticket lifecycle"},
	{ID: "s4", Label: "shell"},
	{ID: "s5", Label: "Wire the auth token", Path: filterWorktree},
	{ID: "s6", Label: "Round-trip tests", Path: filterWorktree},
}

var fixturePhases = map[string]sessionPhase{
	"s1": phaseWaiting, "s2": phaseRunning, "s3": phaseUnread, "s4": phaseIdle, "s5": phaseRunning, "s6": phaseIdle,
}

func phaseOfFixture(id string) sessionPhase { return fixturePhases[id] }
func allIdle(string) sessionPhase           { return phaseIdle }

func picked(phases ...sessionPhase) map[sessionPhase]bool {
	out := map[sessionPhase]bool{}
	for _, p := range phases {
		out[p] = true
	}
	return out
}

func TestFilterSessionsByQuery(t *testing.T) {
	cases := []struct {
		name, query, active string
		want                string
		matched             bool
	}{
		{"blank hands everything back", "   ", "s3", "s1,s2,s3,s4,s5,s6", true},
		// s5 and s6 ride in on their checkout path, which carries "auth-relay".
		{"labels across checkouts", "relay", "s3", "s1,s3,s5,s6", true},
		{"a worktree name", "auth-relay", "s5", "s5,s6", true},
		{"the project path for a root session", "code/lich", "s3", "s1,s2,s3,s4", true},
		{"the active session stays", "relay", "s4", "s1,s3,s4,s5,s6", true},
		{"no match but the active card", "webgl", "s4", "s4", false},
		{"no match and nothing active", "webgl", "", "", false},
		{"case-insensitive tokens", "TICKET relay", "", "s3", true},
	}
	for _, tc := range cases {
		kept, matched := filterSessions(filterFixture, tc.query, filterProject, tc.active, nil, allIdle)
		if idList(kept) != tc.want || matched != tc.matched {
			t.Errorf("%s: kept %s matched %v, want %s %v", tc.name, idList(kept), matched, tc.want, tc.matched)
		}
	}
}

func TestFilterSessionsByPhase(t *testing.T) {
	cases := []struct {
		name, query, active string
		phases              map[sessionPhase]bool
		want                string
		matched             bool
	}{
		{"any picked phase", "", "", picked(phaseWaiting, phaseUnread), "s1,s3", true},
		{"query and phase together", "relay", "", picked(phaseRunning), "s5", true},
		{"the active session whatever its phase", "", "s6", picked(phaseWaiting), "s1,s6", true},
		{"only the active session left", "patch", "s4", picked(phaseWaiting), "s4", false},
	}
	for _, tc := range cases {
		kept, matched := filterSessions(filterFixture, tc.query, filterProject, tc.active, tc.phases, phaseOfFixture)
		if idList(kept) != tc.want || matched != tc.matched {
			t.Errorf("%s: kept %s matched %v, want %s %v", tc.name, idList(kept), matched, tc.want, tc.matched)
		}
	}
}

func TestPhaseOf(t *testing.T) {
	cases := []struct {
		status string
		unread bool
		want   sessionPhase
	}{
		{statusWaiting, false, phaseWaiting},
		{statusBusy, false, phaseRunning},
		{statusCompacting, false, phaseRunning},
		{statusDone, true, phaseUnread},
		{statusDone, false, phaseIdle},
		{statusNone, false, phaseIdle},
	}
	for _, tc := range cases {
		if got := phaseOf(tc.status, tc.unread); got != tc.want {
			t.Errorf("phaseOf(%q, %v) = %q, want %q", tc.status, tc.unread, got, tc.want)
		}
	}
}

func TestPhaseCounts(t *testing.T) {
	all := phaseCounts(filterFixture, "", filterProject, phaseOfFixture)
	if want := map[sessionPhase]int{phaseWaiting: 1, phaseRunning: 2, phaseUnread: 1, phaseIdle: 2}; !reflect.DeepEqual(all, want) {
		t.Fatalf("blank counts = %v, want %v", all, want)
	}
	relay := phaseCounts(filterFixture, "relay", filterProject, phaseOfFixture)
	if want := map[sessionPhase]int{phaseWaiting: 1, phaseRunning: 1, phaseUnread: 1, phaseIdle: 1}; !reflect.DeepEqual(relay, want) {
		t.Fatalf("relay counts = %v, want %v", relay, want)
	}
}

func TestNoMatchNotice(t *testing.T) {
	cases := []struct {
		query  string
		phases map[sessionPhase]bool
		want   string
	}{
		{"", nil, "No sessions. The active session stays."},
		{" webgl ", nil, "No sessions match “webgl”. The active session stays."},
		{"", picked(phaseUnread, phaseWaiting), "No waiting or unread sessions. The active session stays."},
		{"webgl", picked(phaseRunning), "No running sessions match “webgl”. The active session stays."},
	}
	for _, tc := range cases {
		if got := noMatchNotice(tc.query, tc.phases); got != tc.want {
			t.Errorf("noMatchNotice(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}
}
