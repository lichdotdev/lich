package main

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
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
	for _, r := range chatRows(items, claudeKind, true, time.Time{}) {
		key := chatRowKey(r)
		lead[key], live[key] = r.lead, r.live
	}
	if !lead["a1"] || lead["a2"] || !lead["a3"] {
		t.Errorf("lead %v, want a1 and a3 alone", lead)
	}
	// The first turn's call was left running, but that turn is over: only the
	// turn in flight comes alive.
	if live["t1"] || live["a1"] || live["a2"] || !live["a3"] || live["work:1"] {
		t.Errorf("live %v, want the answer being written alone", live)
	}
	running := append(slices.Clone(items), backend.ChatItem{ID: "k2", Kind: api.ItemTool, Turn: 2, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolRunning}})
	for _, r := range chatRows(running, claudeKind, true, time.Time{}) {
		if r.kind == chatRowWork && r.live != (r.turn == 2) {
			t.Errorf("turn %d's group live = %v, want it live only in the turn in flight", r.turn, r.live)
		}
	}
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		if r.live {
			t.Errorf("%s is live in a conversation that is not running", chatRowKey(r))
		}
	}
	items[3].Tool.State = api.ToolDone
	for _, r := range chatRows(items, claudeKind, true, time.Time{}) {
		if r.kind == chatRowWork && r.live {
			t.Error("a group whose calls all ended is live")
		}
	}
	items = append(items[:6:6], backend.ChatItem{ID: "t2", Kind: api.ItemThought, Turn: 2})
	for _, r := range chatRows(items, claudeKind, true, time.Time{}) {
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
	ended := i18n.T("chat.view.workedAt", i18n.Args{"duration": chatTook(0), "count": 1, "time": clockFor(r, novDay(18, 17, 59))})
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
	// The group's line opens the turn, so it starts with the avatar at the
	// column's edge and the calls hang under it, past the connector.
	edge := composerBox(t, r).Min.X + 6
	if tool := labelLeft(t, r, "Read"); tool-edge < 24 {
		t.Errorf("a call's name at x=%d, the column's edge at x=%d: the calls hang under the line, past the connector", tool, edge)
	}
	if line := labelLeft(t, r, i18n.T("chat.view.workedAt", i18n.Args{"duration": chatElapsed(time.Minute), "count": 2, "time": clockFor(r, novDay(18, 17, 59))})); line-edge < 20 {
		t.Errorf("the group's line starts at x=%d: the avatar is not before it (column edge %d)", line, edge)
	}
}

// A call running in a live turn shimmers in its name and target and counts
// its time; once it ended it is still, its time standing at how long it ran.
func TestARunningCallShimmersAndCountsAndAnEndedOneStandsStill(t *testing.T) {
	start := time.Unix(1000, 0)
	call := backend.ChatItem{ID: "k1", Kind: api.ItemTool, At: start.UnixMilli(), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolRunning}}
	draw := func(it backend.ChatItem, live bool) func(C, *ui.Theme) {
		return func(gtx C, th *ui.Theme) { chatTool(gtx, th, it, live) }
	}
	r := newFrameRig(image.Pt(400, 40), draw(call, true))
	r.now = start.Add(2 * time.Second)
	wake := func(it backend.ChatItem, live bool) (time.Time, bool) {
		return wakeAfter(r.now, func(gtx C) { chatTool(gtx, r.theme, it, live) })
	}
	if at, ok := wake(call, true); !ok || at.After(r.now.Add(ui.LoopFrame)) {
		t.Errorf("a running call asked for its next frame at %v (asked %v): it does not shimmer", at, ok)
	}
	r.frame()
	for _, label := range []string{"Bash", "go test", chatElapsed(2 * time.Second)} {
		if !r.shows(label) {
			t.Errorf("no %q:\n%s", label, r.drawn())
		}
	}
	r.now = r.now.Add(3 * time.Second)
	r.frame()
	if !r.shows(chatElapsed(5 * time.Second)) {
		t.Errorf("the time did not count on:\n%s", r.drawn())
	}

	done := call
	done.Tool = &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolDone, Ended: start.Add(4 * time.Second).UnixMilli()}
	r = newFrameRig(image.Pt(400, 40), draw(done, true))
	r.now = start.Add(time.Minute)
	if at, ok := wake(done, true); ok {
		t.Errorf("an ended call asked to be drawn again at %v", at)
	}
	r.frame()
	if !r.shows(chatElapsed(4 * time.Second)) {
		t.Errorf("no time standing at 4s:\n%s", r.drawn())
	}

	// A call whose end nobody reported, read back with the history, shows no
	// time rather than one made up.
	done.Tool = &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolDone}
	r = newFrameRig(image.Pt(400, 40), draw(done, false))
	r.frame()
	if r.shows(chatElapsed(0)) || r.shows(chatTook(0)) {
		t.Errorf("a call with no end shows a time:\n%s", r.drawn())
	}
}

// The band crosses a running call's name and its target, and an ended call's
// row stays as it was drawn.
func TestTheBandCrossesARunningCallsNameAndTarget(t *testing.T) {
	// time.Unix(1000, 0) starts a lap, the band left of the text; half a lap
	// on it stands over the middle of each word, within the same second.
	start := time.Unix(1000, 0)
	size := image.Pt(400, 40)
	// visibleShift is a change of a channel the eye catches, past rounding.
	const visibleShift = 24
	call := func(state string) backend.ChatItem {
		return backend.ChatItem{ID: "k1", Kind: api.ItemTool, At: start.Add(-2 * time.Second).UnixMilli(),
			Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test ./internal/...", State: state, Ended: start.Add(-time.Second).UnixMilli()}}
	}
	for _, tc := range []struct {
		state string
		moves bool
	}{{api.ToolRunning, true}, {api.ToolDone, false}} {
		it := call(tc.state)
		r := newFrameRig(size, func(gtx C, th *ui.Theme) { chatTool(gtx, th, it, true) })
		r.now = start
		r.frame()
		before := renderRig(t, r)
		r.now = start.Add(ui.ShimmerLap / 2)
		r.frame()
		after := renderRig(t, r)
		// A label's bounds run to the row's end, so the name's stop where the
		// target's start.
		target := labelBox(t, r, "go test ./internal/...")
		name := labelBox(t, r, "Bash")
		name.Max.X = target.Min.X
		for label, box := range map[string]image.Rectangle{"Bash": name, "go test ./internal/...": target} {
			if shift := inkShift(before, after, box); (shift >= visibleShift) != tc.moves {
				t.Errorf("%s call: %q shifted by %d between the two frames, moving = %v, want %v", tc.state, label, shift, shift >= visibleShift, tc.moves)
			}
		}
	}
}

// A turn ran until its last call ended, past the last item that began.
func TestAGroupTookUntilItsLastCallEnded(t *testing.T) {
	start := novDay(18, 17, 58)
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: start},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: start + 1000, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolDone, Ended: start + 9000}},
	}
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		if r.kind == chatRowWork && (r.took != 9*time.Second || r.ended != start+9000) {
			t.Errorf("the group took %v and ended at %d, want 9s and %d", r.took, r.ended, start+9000)
		}
	}
}

// While the model is in a run of tool calls its group stands open with every
// call of it, ended and running alike; once it writes text again, or the turn
// is over, the group is shut, and one read back from an ended turn starts so.
func TestAGroupShowsEveryCallWhileTheModelCallsTools(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 58), Tool: &backend.ChatToolCall{Name: "Read", Title: "a.go", State: api.ToolDone}},
		{ID: "k2", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolRunning}},
		{ID: "k3", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Grep", Title: "TODO", State: api.ToolDone}},
	}
	_, r := lifeFrames(t, items, true)
	r.frame()
	for _, label := range []string{"Read", "Bash", "go test", "Grep", "TODO"} {
		if !r.shows(label) {
			t.Errorf("no %q under the group of the run in flight:\n%s", label, r.drawn())
		}
	}
	answered := append(slices.Clone(items), backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "done"})
	for name, c := range map[string]struct {
		items   []backend.ChatItem
		running bool
	}{"text after the run": {answered, true}, "an ended turn": {items, false}} {
		_, r = lifeFrames(t, c.items, c.running)
		r.frame()
		for _, label := range []string{"Read", "Bash", "Grep"} {
			if r.shows(label) {
				t.Errorf("%s: %q stands under a group whose run is over:\n%s", name, label, r.drawn())
			}
		}
	}
}

// A call its turn left running, interrupted, stops with the turn: no band, no
// counter, the time up to the turn's end, or none where that is unknown.
func TestACallOfAnEndedTurnStandsStill(t *testing.T) {
	start := time.Unix(1000, 0)
	call := func(ended int64) backend.ChatItem {
		return backend.ChatItem{ID: "k1", Kind: api.ItemTool, Turn: 1, At: start.UnixMilli(),
			Tool: &backend.ChatToolCall{Name: "Bash", Title: "sleep 9", State: api.ToolRunning, Ended: ended}}
	}
	items := func(it backend.ChatItem) []backend.ChatItem {
		return []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: start.UnixMilli()}, it}
	}
	for _, tc := range []struct {
		ended int64
		want  string
	}{{start.Add(3 * time.Second).UnixMilli(), chatTook(3 * time.Second)}, {0, ""}} {
		it := call(tc.ended)
		var row chatRow
		for _, r := range chatRows(items(it), claudeKind, false, start.Add(time.Minute)) {
			if r.kind == chatRowWork {
				row = r
			}
		}
		r := newFrameRig(image.Pt(400, 40), func(gtx C, th *ui.Theme) { chatTool(gtx, th, it, row.live) })
		r.now = start.Add(time.Minute)
		r.frame()
		if at, ok := wakeAfter(r.now, func(gtx C) { chatTool(gtx, r.theme, it, row.live) }); ok {
			t.Errorf("a call of an ended turn asked to be drawn again at %v", at)
		}
		if tc.want != "" && !r.shows(tc.want) {
			t.Errorf("no %q:\n%s", tc.want, r.drawn())
		}
		if r.shows(chatElapsed(time.Minute)) {
			t.Errorf("the call counts on past its turn:\n%s", r.drawn())
		}
	}
}

// A time that stands still under a second reads in milliseconds.
func TestAStillTimeUnderASecondReadsInMilliseconds(t *testing.T) {
	for d, want := range map[time.Duration]string{476 * time.Millisecond: "476ms", 0: "0ms", 2 * time.Second: chatElapsed(2 * time.Second)} {
		if got := chatTook(d); got != want {
			t.Errorf("chatTook(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestTheFirstAnswerOfATurnIsIntroducedOnceWithTheModelThatWroteIt(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 58), Text: "one", Model: "Haiku 5.5", Effort: "Medium"},
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

// Whichever of the group's line and the first answer opens a turn carries the
// avatar; the other leaves it out.
func TestTheAvatarIsOnTheLineThatOpensTheTurn(t *testing.T) {
	tool := backend.ChatItem{ID: "k1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Read", State: api.ToolDone}}
	answer := backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1}
	flags := func(items ...backend.ChatItem) (work, agent chatRow) {
		for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
			switch r.kind {
			case chatRowWork:
				work = r
			case chatRowItem:
				if r.item.Kind == api.ItemAgent {
					agent = r
				}
			}
		}
		return work, agent
	}
	work, agent := flags(tool, answer)
	if !work.avatar || !agent.bare || !agent.lead {
		t.Errorf("work first: line avatar %v, answer bare %v lead %v; want the line to carry it and the answer's header to leave it out", work.avatar, agent.bare, agent.lead)
	}
	work, agent = flags(answer, tool)
	if work.avatar || agent.bare || !agent.lead {
		t.Errorf("answer first: line avatar %v, answer bare %v; want the answer's header to carry it", work.avatar, agent.bare)
	}
}

func TestEveryLanguageHasVerbsForALiveTurn(t *testing.T) {
	for _, l := range i18n.Locales {
		verbs := strings.Split(i18n.In(l, "chat.view.verbs"), chatVerbSep)
		if len(verbs) < 2 {
			t.Errorf("%s has %d verbs, want a list to pick from", l, len(verbs))
		}
		for _, v := range verbs {
			if strings.TrimSpace(v) == "" || v != strings.TrimSpace(v) {
				t.Errorf("%s has the verb %q", l, v)
			}
		}
	}
}

func TestATurnKeepsItsVerbAndTurnsDifferInTheirs(t *testing.T) {
	first := chatVerb("s1", 1)
	for range 50 {
		if got := chatVerb("s1", 1); got != first {
			t.Fatalf("the turn's verb moved from %q to %q", first, got)
		}
	}
	seen := map[string]bool{}
	for turn := range 40 {
		seen[chatVerb("s1", turn)] = true
	}
	if len(seen) < 4 {
		t.Errorf("40 turns used %d verbs, want the list spread", len(seen))
	}
}

func TestALiveTurnShowsItsVerbWithTheTimeItRunAndNothingMovesOnceItEnds(t *testing.T) {
	items := []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"}}
	_, r := lifeFrames(t, items, true)
	r.frame()
	if verb := chatVerb("s1", 1); !r.shows(verb) {
		t.Errorf("no %q:\n%s", verb, r.drawn())
	}
	if r.shows(i18n.T("chat.view.verbs")) {
		t.Error("the whole list is drawn")
	}
	if at, _ := r.router.WakeupTime(); at.IsZero() {
		t.Error("a live turn asked for no next frame")
	}
	_, r = lifeFrames(t, items, false)
	r.settle(t)
	if r.shows(chatVerb("s1", 1)) {
		t.Error("a turn that is over still names itself")
	}
}

// An answer names the model that wrote it. Setting the chat to another model
// afterwards changes the composer and the answers to come, not the ones
// written.
func TestChangingTheModelLeavesTheNamesOfOldAnswersAlone(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 58), Text: "one", Model: "Haiku 5.5", Effort: "medium"},
	}, false)
	u.model.chat.setOptions("s1", chatOptions())
	r.settle(t)
	if !r.shows("· Haiku 5.5 · medium") {
		t.Fatalf("the answer does not name its model:\n%s", r.drawn())
	}
	switched := chatOptions()
	switched[0].Current, switched[1].Current = "opus", "low"
	u.model.chat.setOptions("s1", switched)
	r.settle(t)
	if !r.shows("· Haiku 5.5 · medium") || r.shows("· Opus 5.5") || r.shows("· Opus 5.5 · Low") {
		t.Errorf("the answer follows the model the chat is set to now:\n%s", r.drawn())
	}
}

func TestAnAnswerNamesWhatItsItemKnows(t *testing.T) {
	for name, c := range map[string]struct {
		model, effort, want string
		absent              []string
	}{
		"both":       {"Haiku 5.5", "medium", "· Haiku 5.5 · medium", nil},
		"model only": {"Haiku 5.5", "", "· Haiku 5.5", []string{"· Haiku 5.5 · "}},
		"neither":    {"", "", "", []string{"· Haiku 5.5", "· Opus 5.5", "· Medium"}},
	} {
		u, r := lifeFrames(t, []backend.ChatItem{
			{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
			{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 58), Text: "one", Model: c.model, Effort: c.effort},
		}, false)
		// The chat is set to a model, which a header must not borrow.
		u.model.chat.setOptions("s1", chatOptions())
		r.settle(t)
		if c.want != "" && !r.shows(c.want) {
			t.Errorf("%s: no %q:\n%s", name, c.want, r.drawn())
		}
		for _, a := range c.absent {
			if r.shows(a) {
				t.Errorf("%s: %q is drawn:\n%s", name, a, r.drawn())
			}
		}
	}
}
