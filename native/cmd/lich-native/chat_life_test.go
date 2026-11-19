package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// wakeAfter draws w once at now and answers the frame it asks for.
func wakeAfter(now time.Time, w func(C)) (time.Time, bool) {
	var router input.Router
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Now: now, Source: router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(200, 200)}}
	w(gtx)
	router.Frame(&ops)
	return router.WakeupTime()
}

func TestTheCounterWakesOnTheSecondItChangesOn(t *testing.T) {
	since := time.Unix(1000, 0)
	at, _ := wakeAfter(since.Add(6300*time.Millisecond), func(gtx C) { chatTick(gtx, since) })
	if want := since.Add(7 * time.Second); !at.Equal(want) {
		t.Errorf("woke at %v, want %v: once a second, not once a frame", at, want)
	}
}

func TestARunningTimeReadsInTheWindowsUnits(t *testing.T) {
	for d, want := range map[time.Duration]string{
		6 * time.Second:                       "6s",
		72 * time.Second:                      "1m 12s",
		time.Hour + 4*time.Minute:             "1h 4m",
		-time.Second:                          "0s",
		59*time.Second + 900*time.Millisecond: "59s",
	} {
		if got := chatElapsed(d); got != want {
			t.Errorf("%v reads %q, want %q", d, got, want)
		}
	}
}

func TestTheCaretBlinksEveryHalfSecondAndWakesOnlyThen(t *testing.T) {
	start := time.UnixMilli(10_000) // a whole half second, an even one
	var on, off string
	at, _ := wakeAfter(start.Add(100*time.Millisecond), func(gtx C) { on = chatCaret(gtx) })
	if !at.Equal(start.Add(500 * time.Millisecond)) {
		t.Errorf("the caret woke at %v, want the next half second", at)
	}
	wakeAfter(start.Add(600*time.Millisecond), func(gtx C) { off = chatCaret(gtx) })
	if on != chatCaretGlyph || off != "" {
		t.Errorf("caret %q then %q, want it shown then gone", on, off)
	}
}

func TestEachToolWearsTheGlyphOfItsKind(t *testing.T) {
	cases := map[string]string{
		"Read": "read", "read": "Read", "Edit": "Write", "MultiEdit": "edit", "Bash": "execute",
		"Grep": "search", "Glob": "Grep", "WebSearch": "search", "WebFetch": "fetch", "think": "think",
	}
	for a, b := range cases {
		if toolIcon(a) != toolIcon(b) {
			t.Errorf("%s and %s wear different glyphs", a, b)
		}
		if toolIcon(a) == icToolOther {
			t.Errorf("%s wears the generic glyph", a)
		}
	}
	if toolIcon("Read") == toolIcon("Bash") || toolIcon("Bash") == toolIcon("Grep") {
		t.Error("a read, a command and a search share a glyph")
	}
	if toolIcon("think") != icThink {
		t.Error("a thought is not the sparkles")
	}
	if toolIcon("SomethingNew") != icToolOther || toolIcon("") != icToolOther {
		t.Error("an unknown tool does not wear the generic glyph")
	}
	if g := toolIcon("mcp__lich__open_session"); g == icToolOther || g != toolIcon("mcp__x__y") {
		t.Error("an MCP tool has no glyph of its own")
	}
}

func TestOnlyTheFirstAnswerOfATurnIsLeadAndOnlyWhileRunningIsAnythingLive(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1},
		{ID: "t1", Kind: api.ItemThought, Turn: 1},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolRunning}},
		{ID: "a2", Kind: api.ItemAgent, Turn: 1},
		{ID: "u2", Kind: api.ItemUser, Turn: 2},
		{ID: "a3", Kind: api.ItemAgent, Turn: 2},
	}
	lead, live := map[string]bool{}, map[string]bool{}
	for _, r := range chatRows(items, true, time.Time{}) {
		key := chatRowKey(r)
		lead[key], live[key] = r.lead, r.live
	}
	if !lead["a1"] || lead["a2"] || !lead["a3"] {
		t.Errorf("lead %v, want a1 and a3 alone", lead)
	}
	if live["t1"] || live["a1"] || live["a2"] || !live["a3"] || !live["work:1"] {
		t.Errorf("live %v, want the answer being written and the group whose call runs", live)
	}
	for _, r := range chatRows(items, false, time.Time{}) {
		if r.live {
			t.Errorf("%s is live in a conversation that is not running", chatRowKey(r))
		}
	}
	items[3].Tool.State = api.ToolDone
	for _, r := range chatRows(items, true, time.Time{}) {
		if r.kind == chatRowWork && r.live {
			t.Error("a group whose calls all ended is live")
		}
	}
	items = append(items[:6:6], backend.ChatItem{ID: "t2", Kind: api.ItemThought, Turn: 2})
	for _, r := range chatRows(items, true, time.Time{}) {
		if r.item.ID == "t2" && !r.live {
			t.Error("the thought being thought is not live")
		}
	}
}

func lifeFrames(t *testing.T, items []backend.ChatItem, running bool) (*window, *frameRig) {
	t.Helper()
	u, r := stampFrames(t)
	u.model.chat.history("s1", items)
	if running {
		u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	}
	return u, r
}

func TestALiveStepAsksForFramesAndAFinishedOneDoesNot(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"},
		{ID: "t1", Kind: api.ItemThought, Turn: 1, At: novDay(18, 17, 59), Text: "hm"},
	}
	_, r := lifeFrames(t, items, true)
	r.frame()
	r.frame() // the first frames settle the conversation onto the composer
	if at, _ := r.router.WakeupTime(); at.IsZero() || !at.After(r.now) || at.After(r.now.Add(time.Second)) {
		t.Errorf("the thought being thought asked for its next frame at %v, want within the second", at)
	}
	_, r = lifeFrames(t, items, false)
	r.settle(t)
}

func TestAThoughtBeingThoughtShimmersAndCountsItsTime(t *testing.T) {
	_, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"},
		{ID: "t1", Kind: api.ItemThought, Turn: 1, At: novDay(18, 17, 59), Text: "hm"},
	}, true)
	r.frame()
	for _, label := range []string{i18n.T("chat.view.thinking"), chatElapsed(r.now.Sub(time.UnixMilli(novDay(18, 17, 59))))} {
		if !r.shows(label) {
			t.Errorf("no %q:\n%s", label, r.drawn())
		}
	}
	if r.shows(i18n.T("chat.view.thought")) {
		t.Error("a thought being thought reads as done")
	}
}

func TestARunningGroupCountsItsCallsAndAFinishedOneSaysWhenItEnded(t *testing.T) {
	call := func(state string) []backend.ChatItem {
		return []backend.ChatItem{
			{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"},
			{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: state}},
		}
	}
	_, r := lifeFrames(t, call(api.ToolRunning), true)
	r.frame()
	if !r.shows(i18n.T("chat.view.toolCalls", i18n.Args{"count": 1})) {
		t.Errorf("no live group line:\n%s", r.drawn())
	}
	_, r = lifeFrames(t, call(api.ToolDone), false)
	r.frame()
	ended := i18n.T("chat.view.workedAt", i18n.Args{"duration": chatElapsed(0), "count": 1, "time": clockFor(r, novDay(18, 17, 59))})
	if !r.shows(ended) {
		t.Errorf("no %q:\n%s", ended, r.drawn())
	}
}

func TestAnOpenGroupLinksItsCallsAndShowsHowEachEnded(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 58), Tool: &backend.ChatToolCall{Name: "Read", Title: "a.go", State: api.ToolDone, Output: "84 lines"}},
		{ID: "k2", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolFailed}},
	}, false)
	u.chats.pane("s1").fold("work:1").open = true
	r.settle(t)
	for _, label := range []string{"Read", "Bash", "84 lines", i18n.T("chat.view.toolFailed")} {
		if !r.shows(label) {
			t.Errorf("no %q:\n%s", label, r.drawn())
		}
	}
	line := labelLeft(t, r, i18n.T("chat.view.workedAt", i18n.Args{"duration": chatElapsed(time.Minute), "count": 2, "time": clockFor(r, novDay(18, 17, 59))}))
	if tool := labelLeft(t, r, "Read"); tool-line < 24 {
		t.Errorf("a call's name at x=%d, the group's line at x=%d: the calls hang under it, past the connector", tool, line)
	}
}

func TestTheFirstAnswerOfATurnIsIntroducedOnceWithTheModelInForce(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 58), Text: "one"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 58), Tool: &backend.ChatToolCall{Name: "Read", State: api.ToolDone}},
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "two"},
	}, false)
	u.model.chat.setOptions("s1", chatOptions())
	r.settle(t)
	count := 0
	for _, n := range r.nodes() {
		if n.Desc.Label == "claude" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the provider is named %d times in a turn of two answers, want once", count)
	}
	if !r.shows("· Haiku 5.5 · Medium") {
		t.Errorf("the model in force is not beside the name:\n%s", r.drawn())
	}
}

func TestAnAnswerBeingWrittenEndsInACaretAndFinishedOnesDoNot(t *testing.T) {
	var live, done string
	wakeAfter(time.UnixMilli(10_100), func(gtx C) {
		live, done = chatSource(gtx, "streaming", true), chatSource(gtx, "streaming", false)
	})
	if live != "streaming"+chatCaretGlyph || done != "streaming" {
		t.Errorf("live %q, finished %q", live, done)
	}
}

func TestANewItemEntersWithMotionAndAHistoryReadLaterDoesNot(t *testing.T) {
	items := userTurns(12)
	u, r := stampFrames(t)
	r.frame()
	u.model.chat.history("s1", items)
	r.frame()
	// The list's first sight of a long conversation scrolls once; after that
	// nothing moves, as it would were every row arriving.
	if _, redraw := r.frame(); redraw {
		t.Error("a history read after the pane opened played as arrivals")
	}
	r.settle(t)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "new", Kind: api.ItemUser, Turn: 13, Text: "fresh"}})
	if _, redraw := r.frame(); !redraw {
		t.Error("a new item appeared at once, with no entry")
	}
	r.settle(t)
	if !r.shows("fresh") {
		t.Error("the new item never settled on screen")
	}
}

// A row coming in grows from no height; the list that was at its end stays
// there while it does, and the rows after it push the earlier ones up.
func TestAListAtItsEndFollowsRowsThatComeIn(t *testing.T) {
	u, r := stampFrames(t)
	r.frame()
	u.model.chat.history("s1", userTurns(60))
	r.settle(t)
	play := func(frames int) {
		for range frames {
			r.now = r.now.Add(16 * time.Millisecond)
			r.frame()
		}
	}
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "new", Kind: api.ItemUser, Turn: 61, Text: "fresh", At: novDay(18, 18, 0)}})
	play(30)
	first := labelTop(t, r, "fresh")
	if first < 0 {
		t.Fatalf("the new message is off screen:\n%s", r.drawn())
	}
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "k", Kind: api.ItemTool, Turn: 61, At: novDay(18, 18, 0), Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolDone}}})
	play(30)
	if later := labelTop(t, r, "fresh"); later < 0 || later >= first {
		t.Errorf("the message at y=%d, then y=%d after a row came in under it: the list stopped following its end", first, later)
	}
}
