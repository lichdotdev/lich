package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestATurnsChangesReadAsItsFiles(t *testing.T) {
	for count, want := range map[int]string{1: "1 file", 3: "3 files"} {
		if got := chatChangesText(api.ChatTurnChanges{Files: count}); got != want {
			t.Errorf("%d files read %q, want %q", count, got, want)
		}
	}
	defer i18n.SetCurrent(i18n.EN)
	i18n.SetCurrent(i18n.RU)
	if got := chatChangesText(api.ChatTurnChanges{Files: 2}); got != "2 файла" {
		t.Errorf("two files in Russian read %q, want its few form", got)
	}
}

func TestOnlyAFinishedTurnThatChangedTheCheckoutGetsAChangesRow(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "n0", Kind: api.ItemNotice, Turn: 0, Text: "resumed"},
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: 1000},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: 2000},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: 3000},
		{ID: "a2", Kind: api.ItemAgent, Turn: 2, At: 4000},
	}
	changed := func(turn int) bool { return turn != 2 }
	turns := func(running bool, changed func(int) bool) (got []int) {
		for _, r := range chatRowsWith(items, running, time.UnixMilli(5000), changed) {
			if r.kind == chatRowDone {
				got = append(got, r.turn)
			}
		}
		return got
	}
	if got := turns(false, changed); len(got) != 1 || got[0] != 1 {
		t.Errorf("changes rows %v, want turn 1 alone: turn 2 changed nothing and the notice opened no turn", got)
	}
	if got := turns(false, func(int) bool { return true }); len(got) != 2 {
		t.Errorf("changes rows %v, want turns 1 and 2", got)
	}
	if got := turns(true, func(int) bool { return true }); len(got) != 1 || got[0] != 1 {
		t.Errorf("changes rows %v while turn 2 runs, want turn 1 alone", got)
	}
	if got := turns(false, nil); got != nil {
		t.Errorf("changes rows %v for a view that knows no changes", got)
	}
	rows := chatRowsWith(items, false, time.UnixMilli(5000), func(int) bool { return true })
	if last := rows[len(rows)-1]; last.kind != chatRowDone || chatRowKey(last) != "changes:2" {
		t.Errorf("the conversation ends in %+v, want turn 2's changes row", last)
	}
}

func TestTheStoreTiesATurnsChangesToTheTurnItClosed(t *testing.T) {
	var s chatStore
	s.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1}, {ID: "u2", Kind: api.ItemUser, Turn: 2}})
	s.turn(api.ChatTurn{SessionID: "s1", Running: true})
	s.turn(api.ChatTurn{SessionID: "s1", StopReason: "end_turn", Changes: &api.ChatTurnChanges{Files: 2, Added: 5}})
	if c, ok := s.changesOf("s1", 2); !ok || c.Files != 2 || c.Added != 5 {
		t.Errorf("turn 2 changed %+v, %v, want the event's count", c, ok)
	}
	if _, ok := s.changesOf("s1", 1); ok {
		t.Error("turn 1, which no event closed, has a count")
	}
	s.turn(api.ChatTurn{SessionID: "s1", StopReason: "end_turn", Changes: &api.ChatTurnChanges{}})
	s.turn(api.ChatTurn{SessionID: "s1", StopReason: "end_turn"})
	if c, _ := s.changesOf("s1", 2); c.Files != 2 {
		t.Errorf("a turn that ended with no count or none changed erased turn 2's: %+v", c)
	}
	s.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1}})
	if _, ok := s.changesOf("s1", 2); ok {
		t.Error("a count outlived the history read over it")
	}
	s.turn(api.ChatTurn{SessionID: "other", StopReason: "end_turn", Changes: &api.ChatTurnChanges{Files: 1}})
	if _, ok := s.changesOf("other", 0); ok {
		t.Error("a conversation the window never heard has a count")
	}
}

// endedTurn is a chat of one finished turn whose end event said end.
func endedTurn(t *testing.T, end api.ChatTurn) (*window, *frameRig) {
	t.Helper()
	u, r := stampFrames(t)
	u.dock = new(dock)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "did it"},
	})
	end.SessionID = "s1"
	u.model.chat.turn(end)
	r.settle(t)
	return u, r
}

func TestAFinishedTurnShowsWhatItChangedAndTheCountOpensReview(t *testing.T) {
	u, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn", Changes: &api.ChatTurnChanges{Files: 3, Added: 12, Removed: 4}})
	for _, label := range []string{"3 files", "+12", "\u22124"} {
		if !r.shows(label) {
			t.Fatalf("no %q:\n%s", label, r.drawn())
		}
	}
	for _, word := range []string{"Done", "Stopped", "Failed"} {
		if r.shows(word) {
			t.Errorf("the line says %q: it carries the changes alone", word)
		}
	}
	r.clickLabel(t, "3 files")
	if u.dock.tab != dockReview {
		t.Errorf("the count opened tab %v, want Review", u.dock.tab)
	}
}

func TestATurnThatChangedNothingOrCouldNotBeReadHasNoLine(t *testing.T) {
	for name, changes := range map[string]*api.ChatTurnChanges{"nothing": {}, "unread": nil} {
		_, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn", Changes: changes})
		for _, label := range []string{"0 files", "1 file", "no changes"} {
			if r.shows(label) {
				t.Errorf("%s: a line says %q:\n%s", name, label, r.drawn())
			}
		}
	}
}
