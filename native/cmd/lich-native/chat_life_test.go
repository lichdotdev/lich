package main

import (
	"image"
	"maps"
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

func TestOnlyWhileRunningIsAnythingLive(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1},
		{ID: "t1", Kind: api.ItemThought, Turn: 1},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolRunning}},
		{ID: "a2", Kind: api.ItemAgent, Turn: 1},
		{ID: "u2", Kind: api.ItemUser, Turn: 2},
		{ID: "a3", Kind: api.ItemAgent, Turn: 2},
	}
	live := map[string]bool{}
	for _, r := range chatRows(items, claudeKind, true, time.Time{}) {
		live[chatRowKey(r)] = r.live
	}
	// The first turn's call was left running, but that turn is over: only the
	// turn in flight comes alive.
	if live["t1"] || live["a1"] || live["a2"] || !live["a3"] || live["work:k1"] {
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

// A group names the call running while it runs, beside how many calls it
// holds; once over it says what its calls did and how long they ran.
func TestARunningGroupCountsItsCallsAndAFinishedOneSaysHowLongTheyRan(t *testing.T) {
	calls := func(state string) []backend.ChatItem {
		return []backend.ChatItem{
			{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"},
			{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Read", Title: "a.go", State: api.ToolDone}},
			{ID: "k2", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: state}},
		}
	}
	_, r := lifeFrames(t, calls(api.ToolRunning), true)
	r.frame()
	for _, s := range []string{"Running go test", "· " + i18n.T("chat.view.toolCalls", i18n.Args{"count": 2})} {
		if !r.shows(s) {
			t.Errorf("no %q on the live group's line:\n%s", s, r.drawn())
		}
	}
	_, r = lifeFrames(t, calls(api.ToolDone), false)
	r.frame()
	for _, s := range []string{"Ran 1 command, read 1 file", chatTook(0)} {
		if !r.shows(s) {
			t.Errorf("no %q:\n%s", s, r.drawn())
		}
	}
}

// An open group lists its calls under its line, past its chevron, each with
// its verb and target; a failed one says why, one that went well nothing more.
func TestAnOpenGroupListsItsCallsAndOnlyAFailureSpeaks(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 58), Tool: &backend.ChatToolCall{Name: "Read", Title: "a.go", State: api.ToolDone, Output: "84 lines"}},
		{ID: "k2", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolFailed}},
	}, false)
	u.chats.pane("s1").fold("work:k1").open = true
	r.settle(t)
	for _, label := range []string{"Read", "a.go", "Ran", "go test", i18n.T("chat.view.toolFailed")} {
		if !r.shows(label) {
			t.Errorf("no %q:\n%s", label, r.drawn())
		}
	}
	if r.shows("84 lines") {
		t.Errorf("a call that went well speaks:\n%s", r.drawn())
	}
	if line, verb := labelLeft(t, r, "Ran 1 command, read 1 file"), labelLeft(t, r, "Read"); verb <= line {
		t.Errorf("a call's verb at x=%d, the group's line at x=%d: the calls do not hang under it", verb, line)
	}
	if read, ran := labelLeft(t, r, "a.go"), labelLeft(t, r, "go test"); read != ran {
		t.Errorf("the targets start at x=%d and x=%d: the verbs are not one column", read, ran)
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
	for _, label := range []string{"Running", "go test", chatElapsed(2 * time.Second)} {
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

// The band crosses a running call's verb and its target, and an ended call's
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
		verb := "Ran"
		if tc.state == api.ToolRunning {
			verb = "Running"
		}
		target := labelBox(t, r, "go test ./internal/...")
		name := labelBox(t, r, verb)
		name.Max.X = target.Min.X
		for label, box := range map[string]image.Rectangle{verb: name, "go test ./internal/...": target} {
			if shift := inkShift(before, after, box); (shift >= visibleShift) != tc.moves {
				t.Errorf("%s call: %q shifted by %d between the two frames, moving = %v, want %v", tc.state, label, shift, shift >= visibleShift, tc.moves)
			}
		}
	}
}

// A run of calls ran from its first call until its last call ended, past the
// last item that began.
func TestAGroupTookUntilItsLastCallEnded(t *testing.T) {
	start := novDay(18, 17, 58)
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: start},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: start + 1000, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolDone, Ended: start + 9000}},
	}
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		if r.kind == chatRowWork && (r.took != 8*time.Second || r.ended != start+9000) {
			t.Errorf("the group took %v and ended at %d, want 8s and %d", r.took, r.ended, start+9000)
		}
	}
}

// A group of calls stands shut while they run, and a click on its line opens
// it with every call of it, ended and running alike; it stays as the person
// left it once text comes or the turn is over.
func TestAGroupStaysShutUntilItsLineIsClicked(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 58), Tool: &backend.ChatToolCall{Name: "Read", Title: "a.go", State: api.ToolDone}},
		{ID: "k2", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test", State: api.ToolRunning}},
		{ID: "k3", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: "Grep", Title: "TODO", State: api.ToolDone}},
	}
	u, r := lifeFrames(t, items, true)
	r.frame()
	for _, label := range []string{"a.go", "go test", "TODO"} {
		if r.shows(label) {
			t.Errorf("%q stands under a group nobody opened:\n%s", label, r.drawn())
		}
	}
	r.queueClick(r.at(t, "Running go test"))
	r.play(15)
	for _, label := range []string{"Read", "a.go", "Running", "go test", "Searched", "TODO"} {
		if !r.shows(label) {
			t.Errorf("no %q under the group a click opened:\n%s", label, r.drawn())
		}
	}
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "done"}})
	r.now = r.now.Add(frameStep)
	r.frame()
	for _, label := range []string{"a.go", "go test", "TODO"} {
		if !r.shows(label) {
			t.Errorf("%q left a group the person opened once text came:\n%s", label, r.drawn())
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

func TestEachAnswerNamesItsModelInItsFootAndNoHeaderNamesTheProvider(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 58), Text: "one", Model: "Haiku 5.5", Effort: "Medium"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 58), Tool: &backend.ChatToolCall{Name: "Read", State: api.ToolDone}},
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "two"},
	}, false)
	u.model.chat.setOptions("s1", chatOptions())
	r.settle(t)
	if r.shows("claude") {
		t.Errorf("an answer is introduced by the provider's name:\n%s", r.drawn())
	}
	overAnswer(r, t, "Haiku 5.5 · Medium · "+clockFor(r, novDay(18, 17, 58)))
	overAnswer(r, t, clockFor(r, novDay(18, 17, 59)))
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
	first := chatVerb("", "s1", 1)
	for range 50 {
		if got := chatVerb("", "s1", 1); got != first {
			t.Fatalf("the turn's verb moved from %q to %q", first, got)
		}
	}
	seen := map[string]bool{}
	for turn := range 40 {
		seen[chatVerb("", "s1", turn)] = true
	}
	if len(seen) < 4 {
		t.Errorf("40 turns used %d verbs, want the list spread", len(seen))
	}
}

func TestTurnsPickFromTheWorkingWordsSetLeavingBlankLinesOut(t *testing.T) {
	custom := "  Pondering \n\n\tMulling\n   \n"
	seen := map[string]bool{}
	for turn := range 40 {
		seen[chatVerb(custom, "s1", turn)] = true
	}
	if want := map[string]bool{"Pondering": true, "Mulling": true}; !maps.Equal(seen, want) {
		t.Errorf("40 turns used %v, want %v", seen, want)
	}
	defaults := chatDefaultVerbs()
	if got := chatVerb(" \n\n", "s1", 1); !slices.Contains(defaults, got) {
		t.Errorf("a setting with no word picked %q, want one of the language's %v", got, defaults)
	}
}

func TestALiveTurnShowsItsVerbWithTheTimeItRunAndNothingMovesOnceItEnds(t *testing.T) {
	items := []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 59), Text: "go"}}
	_, r := lifeFrames(t, items, true)
	r.frame()
	if verb := chatVerb("", "s1", 1); !r.shows(verb) {
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
	if r.shows(chatVerb("", "s1", 1)) {
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
	foot := "Haiku 5.5 · medium · " + clockFor(r, novDay(18, 17, 58))
	overAnswer(r, t, foot)
	switched := chatOptions()
	switched[0].Current, switched[1].Current = "opus", "low"
	u.model.chat.setOptions("s1", switched)
	r.settle(t)
	overAnswer(r, t, foot)
	if r.shows("Opus 5.5 · low · "+clockFor(r, novDay(18, 17, 58))) || r.shows("Opus 5.5") {
		t.Errorf("the answer follows the model the chat is set to now:\n%s", r.drawn())
	}
}

func TestAnAnswerNamesWhatItsItemKnows(t *testing.T) {
	for name, c := range map[string]struct {
		model, effort, lead string
		absent              []string
	}{
		"both":       {"Haiku 5.5", "medium", "Haiku 5.5 · medium · ", nil},
		"model only": {"Haiku 5.5", "", "Haiku 5.5 · ", []string{"Haiku 5.5 · medium · "}},
		"neither":    {"", "", "", []string{"Haiku 5.5", "Opus 5.5", "Medium"}},
	} {
		u, r := lifeFrames(t, []backend.ChatItem{
			{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
			{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 58), Text: "one", Model: c.model, Effort: c.effort},
		}, false)
		// The chat is set to a model, which the foot must not borrow.
		u.model.chat.setOptions("s1", chatOptions())
		r.settle(t)
		clock := clockFor(r, novDay(18, 17, 58))
		overAnswer(r, t, c.lead+clock)
		for _, a := range c.absent {
			if r.shows(a) || r.shows(a+clock) {
				t.Errorf("%s: %q is drawn:\n%s", name, a, r.drawn())
			}
		}
	}
}

func TestThoughtsBackToBackAreOneRowThatKeepsTheFirstID(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1},
		{ID: "t1", Kind: api.ItemThought, Turn: 1, Text: "one"},
		{ID: "t2", Kind: api.ItemThought, Turn: 1, Text: "two"},
		{ID: "k1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolDone}},
		{ID: "t3", Kind: api.ItemThought, Turn: 1, Text: "three"},
	}
	var thoughts []chatRow
	for _, r := range chatRows(items, claudeKind, true, time.Time{}) {
		if r.kind == chatRowItem && r.item.Kind == api.ItemThought {
			thoughts = append(thoughts, r)
		}
	}
	if len(thoughts) != 2 || thoughts[0].item.ID != "t1" || thoughts[0].item.Text != "one\n\ntwo" || thoughts[0].live {
		t.Fatalf("thoughts %+v, want t1 holding \"one\\n\\ntwo\" then t3 after the call", thoughts)
	}
	if thoughts[1].item.ID != "t3" || !thoughts[1].live {
		t.Errorf("last thought %+v, want t3 alive", thoughts[1])
	}
}
