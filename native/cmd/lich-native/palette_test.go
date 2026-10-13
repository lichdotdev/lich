package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/lichclient"
)

// paletteRig is a router with the palette's field focused, and a frame that
// reads the palette's keys the way layoutPalette does.
type paletteRig struct {
	router input.Router
	p      *commandPalette
}

func (r *paletteRig) frame() (enter bool) {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops: ops, Source: r.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(100, 100)),
	}
	enter = r.p.keys(gtx)
	// The field is what takes the focus, as the editor does when it is laid out.
	for {
		if _, ok := gtx.Event(key.FocusFilter{Target: &r.p.query}); !ok {
			break
		}
	}
	gtx.Execute(key.FocusCmd{Tag: &r.p.query})
	event.Op(ops, &r.p.query)
	r.router.Frame(ops)
	return enter
}

func newPaletteRig(total int) *paletteRig {
	r := &paletteRig{p: newCommandPalette(func() {})}
	r.p.total = total
	r.frame()
	r.frame()
	return r
}

func (r *paletteRig) press(name key.Name, mods key.Modifiers) bool {
	r.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	return r.frame()
}

func TestPaletteArrowsWalkTheRowsAndStopAtTheEnds(t *testing.T) {
	r := newPaletteRig(3)
	for i, want := range []int{1, 2, 2} {
		r.press(key.NameDownArrow, 0)
		if r.p.cursor != want {
			t.Fatalf("Down #%d: cursor %d, want %d (no wrap past the last row)", i+1, r.p.cursor, want)
		}
	}
	for i, want := range []int{1, 0, 0} {
		r.press(key.NameUpArrow, 0)
		if r.p.cursor != want {
			t.Fatalf("Up #%d: cursor %d, want %d (no wrap past the first row)", i+1, r.p.cursor, want)
		}
	}
	if !r.p.reveal {
		t.Error("a key move asks for the row to be scrolled into view")
	}
}

// A list that shrank under the cursor leaves the raw index past its end, and
// stepping off that one must move from the row the user sees.
func TestPaletteArrowStepsFromTheClampedCursor(t *testing.T) {
	r := newPaletteRig(5)
	r.p.cursor, r.p.total = 4, 2
	r.press(key.NameUpArrow, 0)
	if r.p.cursor != 0 {
		t.Errorf("Up from row 4 of a list of 2: cursor %d, want 0", r.p.cursor)
	}
	if got := r.p.active(0); got != 0 {
		t.Errorf("active of an empty list = %d, want 0", got)
	}
}

func TestPaletteTabWalksTheFiltersAndWraps(t *testing.T) {
	r := newPaletteRig(1)
	r.press(key.NameTab, 0)
	if r.p.tab != tabSessions {
		t.Errorf("Tab from All: %s, want Sessions", r.p.tab)
	}
	r.press(key.NameTab, key.ModShift)
	r.press(key.NameTab, key.ModShift)
	if r.p.tab != tabHistory {
		t.Errorf("Shift+Tab twice from Sessions: %s, want History (wrapped)", r.p.tab)
	}
}

// An action search has nothing to filter by kind, so the key is swallowed
// rather than walking tabs nobody can see.
func TestPaletteTabIsDeadWhileSearchingActions(t *testing.T) {
	r := newPaletteRig(1)
	r.p.query.SetText("> pin")
	r.press(key.NameTab, 0)
	if r.p.tab != tabAll {
		t.Errorf("Tab in action mode moved to %s", r.p.tab)
	}
}

func TestPaletteEnterIsReported(t *testing.T) {
	r := newPaletteRig(1)
	if !r.press(key.NameReturn, 0) {
		t.Error("Enter was not reported")
	}
	if r.press(key.NameDownArrow, 0) {
		t.Error("an arrow was reported as Enter")
	}
}

func TestPaletteHistoryPollsOnlyWhileIndexingBehindATerm(t *testing.T) {
	p := newCommandPalette(func() {})
	now := time.Unix(1_700_000_000, 0)
	p.history.data = historyAnswer{indexing: 3}
	if got := p.pollHistory(now, ""); got != 0 || !p.pollAt.IsZero() {
		t.Errorf("no term: polls %d, due %v; the backfill does not run without one", got, p.pollAt)
	}
	if got := p.pollHistory(now, "relay"); got != 0 || !p.pollAt.Equal(now.Add(paletteBackfillPoll)) {
		t.Errorf("first ask: polls %d, due %v; want 0 and a second out", got, p.pollAt)
	}
	if got := p.pollHistory(now.Add(paletteBackfillPoll/2), "relay"); got != 0 {
		t.Errorf("before it is due: polls %d, want 0", got)
	}
	if got := p.pollHistory(now.Add(paletteBackfillPoll), "relay"); got != 1 || !p.pollAt.IsZero() {
		t.Errorf("when due: polls %d, due %v; want 1 and no pending poll", got, p.pollAt)
	}
	p.history.data = historyAnswer{}
	if got := p.pollHistory(now, "relay"); got != 1 || !p.pollAt.IsZero() {
		t.Errorf("indexed: polls %d, due %v; it stops asking", got, p.pollAt)
	}
}

func workspaceForView() paletteWorld {
	projects := paletteWorkspace()
	return paletteWorld{
		projects: []paletteProject{{ID: "p1", Name: "lich", Path: "/home/u/try/skipo"}, {ID: "p2", Name: "revu", Path: "/home/u/try/revu"}},
		sessions: paletteSessionsOf(projects),
		counts:   map[string]int{"p1": 2, "p2": 1},
	}
}

func TestPaletteViewJoinsTheWorkspaceToTheAnswers(t *testing.T) {
	p := newCommandPalette(func() {})
	p.show(map[string]bool{"s3": true})
	p.closed = closedHolding(closedAnswer{rows: paletteClosed, total: 63, missing: map[string]bool{"/home/u/snk/gh-turnkey": true}})
	p.history.data = historyAnswer{rows: historyRows(paletteParked, paletteBranches, nil), total: 143, indexing: 12}
	p.said.data = []lichclient.TranscriptMatch{{ID: "s1", Snippet: "flaky again", Count: 2}}

	v := p.view(workspaceForView(), defaultHotkeys(), false, func(hotkeyID) bool { return true })
	equal(t, "All", groupLabels(v.groups), []string{"Sessions", "Projects", "Messages"})
	equal(t, "ranked: the running session leads, the shell is last", rowKeys(v.groups)[:3], []string{"s3", "s1", "s2"})
	if v.counts[tabProjects] != 6 || v.counts[tabHistory] != 3 || v.counts[tabMessages] != 1 {
		t.Errorf("counts = %v", v.counts)
	}
	if _, ok := v.counts[tabAll]; ok {
		t.Error("All reports no count")
	}
	if !v.missing["/home/u/snk/gh-turnkey"] || v.sessionCount["p1"] != 2 {
		t.Errorf("rows report themselves: missing %v, sessions %v", v.missing, v.sessionCount)
	}

	p.tab = tabHistory
	v = p.view(workspaceForView(), defaultHotkeys(), false, func(hotkeyID) bool { return true })
	if v.groups[0].total != 143 || v.groups[0].note != "" {
		t.Errorf("History group: %d of %d, note %q; a term-less list says nothing about indexing", len(v.groups[0].rows), v.groups[0].total, v.groups[0].note)
	}
	p.query.SetText("relay")
	v = p.view(workspaceForView(), defaultHotkeys(), false, func(hotkeyID) bool { return true })
	if v.groups[0].note != "indexing 12 sessions" {
		t.Errorf("note behind a term = %q", v.groups[0].note)
	}
}

func TestPaletteForgottenRowsLeaveTheListAndItsCount(t *testing.T) {
	p := newCommandPalette(func() {})
	p.show(nil)
	p.tab = tabHistory
	p.history.data = historyAnswer{rows: historyRows(paletteParked, paletteBranches, nil), total: 143}
	p.dropped["h2"] = true
	v := p.view(workspaceForView(), defaultHotkeys(), false, func(hotkeyID) bool { return true })
	equal(t, "rows", rowKeys(v.groups), []string{"h1", "h3"})
	if v.groups[0].total != 142 {
		t.Errorf("total = %d, want 142: the count comes down with the row", v.groups[0].total)
	}
}

func TestPaletteHistoryOfAWorkspaceThatClosedNothingSaysSo(t *testing.T) {
	p := newCommandPalette(func() {})
	p.show(nil)
	p.tab = tabHistory
	if v := p.view(workspaceForView(), defaultHotkeys(), false, func(hotkeyID) bool { return true }); !v.emptyHistory {
		t.Error("an empty query on an empty History is a workspace that closed nothing")
	}
	p.query.SetText("relay")
	if v := p.view(workspaceForView(), defaultHotkeys(), false, func(hotkeyID) bool { return true }); v.emptyHistory {
		t.Error("with a term typed an empty list is a search that missed, never an empty workspace")
	}
}

func TestPaletteActionModeListsOnlyWhatTheWindowCanRun(t *testing.T) {
	p := newCommandPalette(func() {})
	p.show(nil)
	p.query.SetText("> ")
	only := func(id hotkeyID) bool { return id == hotkeyTogglePin || id == hotkeyCommandPalette }
	v := p.view(workspaceForView(), defaultHotkeys(), false, only)
	if !v.actionMode {
		t.Fatal("a leading > is the action search")
	}
	equal(t, "rows", rowKeys(v.groups), []string{"action:togglePin"})
}

func TestRowVerbNamesWhatEnterDoes(t *testing.T) {
	rows := historyRows(paletteParked, paletteBranches, map[string]bool{"/home/u/wt/revu/gone": true})
	for _, c := range []struct {
		row  paletteRow
		want string
	}{
		{paletteRow{kind: rowSession}, "open"},
		{paletteRow{kind: rowClosed}, "open"},
		{paletteRow{kind: rowHistory, history: rows[0]}, "resume"},
		{paletteRow{kind: rowHistory, history: rows[2]}, "forget"},
		{paletteRow{kind: rowExternal}, "resume"},
		{paletteRow{kind: rowAction}, "run"},
	} {
		if got := rowVerb(c.row); got != c.want {
			t.Errorf("rowVerb(kind %d) = %q, want %q", c.row.kind, got, c.want)
		}
	}
}
