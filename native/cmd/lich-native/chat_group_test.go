package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// callRun is a run of three tool calls in turn 1, at step: each call is
// asked for, runs and ends before the next.
var callRun = []struct{ id, name, title string }{
	{"k1", "Edit", "a.go"}, {"k2", "Bash", "go test"}, {"k3", "Grep", "TODO"},
}

func runCall(i int, state string) backend.ChatItem {
	c := callRun[i]
	return backend.ChatItem{ID: c.id, Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59), Tool: &backend.ChatToolCall{Name: c.name, Title: c.title, State: state}}
}

// A run of three calls keeps its group shut in every frame, calls coming in
// included, until the person clicks its line.
func TestARunOfCallsKeepsItsGroupShutUntilClicked(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"}}, true)
	r.frames(2)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: runCall(0, api.ToolRunning)})
	r.play(15)
	if r.shows("Edit") {
		t.Fatalf("the run's first call stands under a group nobody opened:\n%s", r.drawn())
	}
	r.queueClick(r.at(t, i18n.T("chat.view.toolCalls", i18n.Args{"count": 1})))
	r.play(15)
	if !r.shows("Edit") {
		t.Fatalf("the group did not open on a click:\n%s", r.drawn())
	}
	for i := 1; i < len(callRun); i++ {
		u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: runCall(i, api.ToolRunning)})
		r.play(15)
		for _, j := range []int{0, i} {
			if !r.shows(callRun[j].name) {
				t.Errorf("%q left the group a click opened:\n%s", callRun[j].name, r.drawn())
			}
		}
	}
	for i := range callRun {
		u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: runCall(i, api.ToolDone)})
	}
	r.play(15)
	for _, c := range callRun {
		if !r.shows(c.name) {
			t.Errorf("%q left the group once its run was over:\n%s", c.name, r.drawn())
		}
	}
}

// Folding a group by hand keeps it folded through a call that comes after,
// and it stays folded once the run is over.
func TestAGroupFoldedByHandStaysFolded(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		runCall(0, api.ToolRunning),
	}, true)
	r.frames(2)
	label := i18n.T("chat.view.toolCalls", i18n.Args{"count": 1})
	r.queueClick(r.at(t, label))
	r.play(15)
	if !r.shows("Edit") {
		t.Fatalf("the group did not open on a click:\n%s", r.drawn())
	}
	r.queueClick(r.at(t, label))
	r.play(15)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: runCall(1, api.ToolRunning)})
	r.play(15)
	if r.shows("Edit") || r.shows("Bash") {
		t.Errorf("the group folded by hand opened again for a call:\n%s", r.drawn())
	}
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: runCall(0, api.ToolDone)})
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: runCall(1, api.ToolDone)})
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "ok"}})
	r.play(15)
	if r.shows("Edit") || r.shows("Bash") {
		t.Errorf("the group opened once the run was over:\n%s", r.drawn())
	}
}
