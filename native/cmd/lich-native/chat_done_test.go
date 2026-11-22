package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestADoneLineNamesHowTheTurnEnded(t *testing.T) {
	for reason, want := range map[string]string{
		"end_turn":   "Done",
		"cancelled":  "Stopped",
		"error":      "Failed",
		"max_tokens": "Done",
		"":           "Done",
	} {
		if got := chatDoneWord(reason); got != want {
			t.Errorf("a turn that stopped for %q reads %q, want %q", reason, got, want)
		}
	}
}

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

func TestOnlyTheLastFinishedTurnEndsInADoneRow(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "n0", Kind: api.ItemNotice, Turn: 0, Text: "resumed"},
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: 1000},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: 2000},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: 3000},
		{ID: "a2", Kind: api.ItemAgent, Turn: 2, At: 4000},
	}
	done := func(items []backend.ChatItem, running bool) (turns []int) {
		for _, r := range chatRows(items, claudeKind, running, time.UnixMilli(5000)) {
			if r.kind == chatRowDone {
				turns = append(turns, r.turn)
			}
		}
		return turns
	}
	if got := done(items, false); len(got) != 1 || got[0] != 2 {
		t.Errorf("done rows %v for a conversation of two finished turns, want turn 2 alone", got)
	}
	rows := chatRows(items, claudeKind, false, time.UnixMilli(5000))
	if last := rows[len(rows)-1]; last.kind != chatRowDone || chatRowKey(last) != "done:2" {
		t.Errorf("the conversation ends in %+v, want turn 2's done row", last)
	}
	if got := done(items, true); got != nil {
		t.Errorf("done rows %v while turn 2 runs, want none: the live row stands there", got)
	}
	// The person's next message takes the line away: turn 3 has begun.
	next := append(items[:5:5], backend.ChatItem{ID: "u3", Kind: api.ItemUser, Turn: 3, At: 6000})
	if got := done(next, true); got != nil {
		t.Errorf("done rows %v once turn 3 began, want none", got)
	}
	if got := done(items[:1], false); got != nil {
		t.Errorf("done rows %v for a conversation no message opened", got)
	}
	if got := done(nil, false); got != nil {
		t.Errorf("done rows %v for an empty conversation", got)
	}
}

func TestTheStoreTiesATurnEndToTheTurnItClosed(t *testing.T) {
	var s chatStore
	s.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1}, {ID: "u2", Kind: api.ItemUser, Turn: 2}})
	s.turn(api.ChatTurn{SessionID: "s1", Running: true})
	s.turn(api.ChatTurn{SessionID: "s1", StopReason: "cancelled", Changes: &api.ChatTurnChanges{Files: 1}})
	if end, ok := s.endOf("s1", 2); !ok || end.StopReason != "cancelled" || end.Changes.Files != 1 {
		t.Errorf("turn 2 ended %+v, %v, want the event's end", end, ok)
	}
	if _, ok := s.endOf("s1", 1); ok {
		t.Error("turn 1, which no event closed, has an end")
	}
	s.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1}})
	if _, ok := s.endOf("s1", 2); ok {
		t.Error("an end outlived the history read over it")
	}
	s.turn(api.ChatTurn{SessionID: "other", StopReason: "end_turn"})
	if _, ok := s.endOf("other", 0); ok {
		t.Error("a conversation the window never heard has an end")
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

func TestTheDoneLineShowsWhatTheTurnChangedAndTheCountOpensReview(t *testing.T) {
	u, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn", Changes: &api.ChatTurnChanges{Files: 3, Added: 12, Removed: 4}})
	for _, label := range []string{"Done", "3 files", "+12", "\u22124"} {
		if !r.shows(label) {
			t.Fatalf("no %q:\n%s", label, r.drawn())
		}
	}
	r.clickLabel(t, "3 files")
	if u.dock.tab != dockReview {
		t.Errorf("the count opened tab %v, want Review", u.dock.tab)
	}
}

func TestTheDoneLineOfATurnThatChangedNothingIsNotAClick(t *testing.T) {
	u, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn", Changes: &api.ChatTurnChanges{}})
	if !r.shows("Done") || !r.shows("no changes") {
		t.Fatalf("want Done, no changes:\n%s", r.drawn())
	}
	r.clickLabel(t, "no changes")
	if u.dock.tab == dockReview {
		t.Error("\"no changes\" opened Review")
	}
}

func TestTheDoneLineOfAStoppedAndAFailedTurn(t *testing.T) {
	for reason, want := range map[string]string{"cancelled": "Stopped", "error": "Failed"} {
		_, r := endedTurn(t, api.ChatTurn{StopReason: reason, Changes: &api.ChatTurnChanges{Files: 1, Added: 1}})
		if !r.shows(want) || r.shows("Done") || !r.shows("1 file") {
			t.Errorf("a turn that stopped for %q:\n%s", reason, r.drawn())
		}
	}
}

func TestTheDoneLineWithoutACountIsItsWordAlone(t *testing.T) {
	_, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn"})
	if !r.shows("Done") || r.shows("no changes") || r.shows("0 files") {
		t.Errorf("a turn whose checkout could not be read:\n%s", r.drawn())
	}
}

func TestTheDoneLineOfATurnReadBackWithTheHistoryIsDoneAlone(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "did it"},
	})
	r.settle(t)
	if !r.shows("Done") || r.shows("no changes") {
		t.Errorf("a turn nobody reported the end of:\n%s", r.drawn())
	}
}

func TestAHistoryOfSeveralTurnsClosesOnlyTheLast(t *testing.T) {
	u, r := stampFrames(t)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 50), Text: "one"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 51), Text: "first"},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 17, 58), Text: "two"},
		{ID: "a2", Kind: api.ItemAgent, Turn: 2, At: novDay(18, 17, 59), Text: "second"},
	})
	r.settle(t)
	if n := strings.Count(r.drawn(), `"Done"`); n != 1 {
		t.Errorf("%d done lines over two turns read back, want the last turn's alone:\n%s", n, r.drawn())
	}
}

func TestTheNextMessageTakesTheDoneLineAway(t *testing.T) {
	u, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn", Changes: &api.ChatTurnChanges{Files: 3, Added: 12, Removed: 4}})
	if !r.shows("Done") {
		t.Fatalf("no done line before the message:\n%s", r.drawn())
	}
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 18, 0), Text: "next"}})
	for range 60 {
		r.now = r.now.Add(frameStep)
		r.frame()
	}
	if r.shows("Done") || r.shows("3 files") {
		t.Errorf("the line is still there once the next message was sent:\n%s", r.drawn())
	}
}

// The line does not play an exit: the turn that begins takes it away in the
// frame it is heard, or it would stand beside the live line for a moment.
func TestTheDoneLineLeavesInTheFrameTheNextTurnBegins(t *testing.T) {
	for name, begin := range map[string]func(u *window){
		"the turn is heard first": func(u *window) { u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true}) },
		"the message is heard first": func(u *window) {
			u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "u2", Kind: api.ItemUser, Turn: 2, At: novDay(18, 18, 0), Text: "next"}})
		},
	} {
		u, r := endedTurn(t, api.ChatTurn{StopReason: "end_turn", Changes: &api.ChatTurnChanges{Files: 3, Added: 12, Removed: 4}})
		if !r.shows("Done") {
			t.Fatalf("%s: no done line before the next turn:\n%s", name, r.drawn())
		}
		begin(u)
		r.now = r.now.Add(frameStep)
		r.frame()
		if r.shows("Done") || r.shows("3 files") {
			t.Errorf("%s: the line is drawn in the frame the next turn began:\n%s", name, r.drawn())
		}
	}
}
