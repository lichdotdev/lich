package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func planItems(turn int, entries ...api.ChatPlanEntry) []backend.ChatItem {
	return []backend.ChatItem{
		{ID: "user-1", Kind: api.ItemUser, Turn: 1, Text: "make dates take ISO 8601"},
		{ID: "a/0", Kind: api.ItemAgent, Turn: 1, Text: "Reading the parser first."},
		{ID: "plan-" + strconv.Itoa(turn), Kind: api.ItemPlan, Turn: turn, Plan: &api.ChatPlan{Entries: entries}},
	}
}

var fivePlan = []api.ChatPlanEntry{
	{Content: "Read the parser", Status: api.PlanCompleted},
	{Content: "Accept a date with no offset", Status: api.PlanCompleted},
	{Content: "Accept +hh:mm and Z", ActiveForm: "Accepting +hh:mm and Z", Status: api.PlanInProgress},
	{Content: "Cover the cases in tests", Status: api.PlanPending},
	{Content: "Update the README", Status: api.PlanPending},
}

func planFrames(t *testing.T, items []backend.ChatItem) *frameRig {
	t.Helper()
	return planFramesRunning(t, items, true)
}

// planFramesRunning draws items on the theme's ground, so a shot reads as the
// window does, with a turn running or not.
func planFramesRunning(t *testing.T, items []backend.ChatItem, running bool) *frameRig {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", items)
	if running {
		u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	}
	draw := r.draw
	r.draw = func(gtx C, th *ui.Theme) {
		paint.Fill(gtx.Ops, th.Background)
		draw(gtx, th)
	}
	r.frame()
	return r
}

// The folded line shows the task running, in its words for while it runs,
// and the count, and none of the other tasks.
func TestThePlanLineShowsTheCountAndTheTaskRunning(t *testing.T) {
	r := planFrames(t, planItems(1, fivePlan...))
	if !r.showsAll("2/5", "Accepting +hh:mm and Z") {
		t.Fatalf("the plan line is not drawn:\n%s", r.drawn())
	}
	if r.shows("Accept a date with no offset") || r.shows("Cover the cases in tests") {
		t.Errorf("the folded line drew more than the task running:\n%s", r.drawn())
	}
	writeShot(t, r, "plan-folded.png")
}

// A click unfolds the task before the one running, it and the next, and
// counts the rest; a second click folds it back.
func TestAClickUnfoldsThePlanAndASecondFoldsIt(t *testing.T) {
	r := planFrames(t, planItems(1, fivePlan...))
	r.click(t, r.at(t, "2/5"))
	if !r.showsAll("Accept a date with no offset", "Accepting +hh:mm and Z", "Cover the cases in tests", "+2 more · 2 of 5 done") {
		t.Fatalf("the click did not unfold the list:\n%s", r.drawn())
	}
	if r.shows("Read the parser") || r.shows("Update the README") {
		t.Errorf("the unfolded list drew past the task after the one running:\n%s", r.drawn())
	}
	writeShot(t, r, "plan-open.png")
	r.click(t, r.at(t, "Accepting +hh:mm and Z"))
	if r.shows("Cover the cases in tests") {
		t.Errorf("a second click left the list open:\n%s", r.drawn())
	}
}

// The unfolded list keeps three tasks where the list has them, sliding back
// from its end and starting at its first.
func TestThePlanWindowKeepsThreeTasks(t *testing.T) {
	at := func(statuses ...string) []api.ChatPlanEntry {
		var es []api.ChatPlanEntry
		for _, s := range statuses {
			es = append(es, api.ChatPlanEntry{Status: s})
		}
		return es
	}
	done, run, todo := api.PlanCompleted, api.PlanInProgress, api.PlanPending
	cases := []struct {
		name     string
		entries  []api.ChatPlanEntry
		from, to int
	}{
		{"first running", at(run, todo, todo, todo), 0, 3},
		{"middle running", at(done, done, run, todo, todo), 1, 4},
		{"last running", at(done, done, done, run), 1, 4},
		{"none running", at(done, todo, todo, todo), 0, 3},
		{"two tasks", at(done, run), 0, 2},
	}
	for _, c := range cases {
		if from, to := planWindow(c.entries); from != c.from || to != c.to {
			t.Errorf("%s: window %d..%d, want %d..%d", c.name, from, to, c.from, c.to)
		}
	}
}

// A list with every task done, a cleared one and the last turn's list in a
// new turn all draw nothing.
func TestThePlanGoesWhenNothingIsLeftOrTheTurnMovesOn(t *testing.T) {
	done := []api.ChatPlanEntry{{Content: "Read the parser", Status: api.PlanCompleted}}
	cases := map[string][]backend.ChatItem{
		"all done": planItems(1, done...),
		"cleared":  planItems(1),
		"new turn": append(planItems(1, fivePlan...), backend.ChatItem{ID: "user-2", Kind: api.ItemUser, Turn: 2, Text: "and now?"}),
	}
	for name, items := range cases {
		t.Run(name, func(t *testing.T) {
			r := planFrames(t, items)
			if r.shows("0/1") || r.shows("1/1") || r.shows("2/5") || r.shows("0/0") {
				t.Errorf("the plan is drawn:\n%s", r.drawn())
			}
		})
	}
}

// A list left with tasks once the turn ended stays, still: nothing shimmers
// or waves and the window asks for no frames.
func TestAPlanLeftAfterTheTurnStaysStill(t *testing.T) {
	r := planFramesRunning(t, planItems(1, fivePlan...), false)
	r.settle(t)
	if !r.showsAll("2/5", "Accepting +hh:mm and Z") {
		t.Fatalf("the plan line is not drawn after the turn:\n%s", r.drawn())
	}
}

// The plan item is no row of the conversation: the live line stays the last.
func TestThePlanItemIsNoConversationRow(t *testing.T) {
	rows := chatRows(planItems(1, fivePlan...), claudeKind, true, time.Time{})
	for _, r := range rows {
		if r.kind == chatRowItem && r.item.Kind == api.ItemPlan {
			t.Fatal("the plan item became a conversation row")
		}
	}
	if rows[len(rows)-1].kind != chatRowLive {
		t.Errorf("last row %v, want the live line", rows[len(rows)-1].kind)
	}
}

// writeShot writes r's frame to LICH_SHOT_DIR, when set, for a person to look at.
func writeShot(t *testing.T, r *frameRig, name string) {
	t.Helper()
	dir := os.Getenv("LICH_SHOT_DIR")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, renderRig(t, r)); err != nil {
		t.Fatal(err)
	}
}
