package tray

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

var testLabels = Labels{
	NeedsYou: "Needs you", Working: "Working", Idle: "Idle: {count}", More: "{count} more",
	Waiting: "Waiting", Done: "Done",
}

func statuses(m map[string]api.SessionStatus) func(string) api.SessionStatus {
	return func(id string) api.SessionStatus { return m[id] }
}

func TestSessionViewSortsByWhatTheSessionNeeds(t *testing.T) {
	live := []api.Peer{
		{ID: "w", Label: "Fix login", Project: "api"},
		{ID: "d", Label: "Tray labels", Project: "lich"},
		{ID: "r", Label: "Read turn", Project: "lich"},
		{ID: "b", Label: "Refactor", Project: "lich"},
		{ID: "c", Name: "claude-2"},
		{ID: "i", Label: "Quiet"},
	}
	v := buildSessionView(live, statuses(map[string]api.SessionStatus{
		"w": {State: "waiting", Reason: "Allow Bash?"},
		"d": {State: "done"},
		"r": {State: "done"},
		"b": {State: "busy", Tool: "Edit"},
		"c": {State: "compacting"},
	}), map[string]bool{"d": true}, testLabels)

	needs := []sessionRow{{"w", "● api · Fix login · Allow Bash?", "● api · Fix login · Allow Bash?"}, {"d", "● lich · Tray labels · Done", "● lich · Tray labels · Done"}}
	if fmt.Sprint(v.needs) != fmt.Sprint(needs) {
		t.Errorf("needs = %v, want %v", v.needs, needs)
	}
	working := []sessionRow{{"b", "◐ lich · Refactor · Edit", "◐ lich · Refactor · Edit"}, {"c", "◐ claude-2", "◐ claude-2"}}
	if fmt.Sprint(v.working) != fmt.Sprint(working) {
		t.Errorf("working = %v, want %v", v.working, working)
	}
	if v.idle != 2 {
		t.Errorf("idle = %d, want the read turn and the quiet one", v.idle)
	}
	if v.badge != badgeWaiting {
		t.Errorf("badge = %v, want waiting", v.badge)
	}
	tip := "lich\nNeeds you\n● api · Fix login · Allow Bash?\n● lich · Tray labels · Done\nWorking\n◐ lich · Refactor · Edit\n◐ claude-2\nIdle: 2"
	if v.tooltip != tip {
		t.Errorf("tooltip = %q, want %q", v.tooltip, tip)
	}
}

func TestSessionViewWaitingWithNoReasonSaysWaiting(t *testing.T) {
	v := buildSessionView([]api.Peer{{ID: "w", Label: "a"}}, statuses(map[string]api.SessionStatus{"w": {State: "waiting"}}), nil, testLabels)
	if len(v.needs) != 1 || v.needs[0].title != "● a · Waiting" {
		t.Errorf("needs = %v", v.needs)
	}
}

func TestSessionViewCountsPastTheSlots(t *testing.T) {
	var live []api.Peer
	busy := map[string]api.SessionStatus{}
	for i := range sessionSlots + 3 {
		id := fmt.Sprint(i)
		live = append(live, api.Peer{ID: id, Label: id})
		busy[id] = api.SessionStatus{State: "busy"}
	}
	v := buildSessionView(live, statuses(busy), nil, testLabels)
	if len(v.working) != sessionSlots || v.workingMore != 3 {
		t.Errorf("working %d, more %d; want %d and 3", len(v.working), v.workingMore, sessionSlots)
	}
	if got := strings.Count(v.tooltip, markWorking); got != sessionSlots+3 {
		t.Errorf("tooltip lists %d working sessions, want every one: %q", got, v.tooltip)
	}
}

func TestBadgeRanksWaitingThenUnreadThenBusy(t *testing.T) {
	cases := []struct {
		waiting, done, busy bool
		want                badge
	}{
		{true, true, true, badgeWaiting},
		{false, true, true, badgeDone},
		{false, false, true, badgeBusy},
		{false, false, false, badgeNone},
	}
	for _, c := range cases {
		if got := badgeOf(c.waiting, c.done, c.busy); got != c.want {
			t.Errorf("badgeOf(%v, %v, %v) = %v, want %v", c.waiting, c.done, c.busy, got, c.want)
		}
	}
}

func TestSessionRowClipsTheTitleAndKeepsTheFullLine(t *testing.T) {
	name, detail := strings.Repeat("a", 50), strings.Repeat("b", 80)
	row := newRow("x", markWorking, api.Peer{Label: name, Project: "p"}, detail)
	want := markWorking + "p · " + strings.Repeat("a", labelRunes-1) + "… · " + strings.Repeat("b", detailRunes-1) + "…"
	if row.title != want {
		t.Errorf("title = %q, want %q", row.title, want)
	}
	if full := markWorking + "p · " + name + " · " + detail; row.full != full {
		t.Errorf("full = %q, want %q", row.full, full)
	}
}

func TestSessionSlotsFocusWhatTheyShowed(t *testing.T) {
	tr := New(Icons{}, func() {}, func() error { return nil })
	if v, err := tr.readSessions(testLabels); err != nil || v.any() {
		t.Fatalf("without sessions wired: %+v, %v", v, err)
	}
	live := []api.Peer{{ID: "a"}, {ID: "b"}}
	var focused []string
	tr.SetSessions(Sessions{
		Live:   func() ([]api.Peer, error) { return live, nil },
		Status: statuses(map[string]api.SessionStatus{"a": {State: "busy"}, "b": {State: "busy"}}),
		Unread: func() (map[string]bool, error) { return nil, nil },
		Focus:  func(id string) { focused = append(focused, id) },
	})
	if _, err := tr.readSessions(testLabels); err != nil {
		t.Fatal(err)
	}
	tr.focusSlot(&tr.workIDs, 1)
	live = live[:1]
	if _, err := tr.readSessions(testLabels); err != nil {
		t.Fatal(err)
	}
	tr.focusSlot(&tr.workIDs, 1)
	tr.focusSlot(&tr.needIDs, 0)
	if fmt.Sprint(focused) != "[b]" {
		t.Errorf("focused %v, want only the session slot 1 showed", focused)
	}
}

func TestReadSessionsPassesAReadErrorOn(t *testing.T) {
	tr := New(Icons{}, func() {}, func() error { return nil })
	fail := errors.New("store gone")
	tr.SetSessions(Sessions{
		Live:   func() ([]api.Peer, error) { return nil, nil },
		Unread: func() (map[string]bool, error) { return nil, fail },
	})
	if _, err := tr.readSessions(testLabels); !errors.Is(err, fail) {
		t.Errorf("err = %v, want the unread read's", err)
	}
}

func TestSwapBadgeReportsOnlyAChange(t *testing.T) {
	tr := New(Icons{}, func() {}, func() error { return nil })
	if tr.swapBadge(badgeNone) {
		t.Error("the badge the icon starts with reported as a change")
	}
	if !tr.swapBadge(badgeBusy) || tr.swapBadge(badgeBusy) {
		t.Error("want one change for one new badge")
	}
}
