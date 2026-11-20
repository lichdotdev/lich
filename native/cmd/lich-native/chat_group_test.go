package main

import (
	"slices"
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

// A run of three calls keeps every call of it in view while it runs and
// between one call and the next, in every frame; the group shuts only when
// the model writes text again, by the fold's motion.
func TestARunOfCallsStaysInViewUntilTextComes(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"}}, true)
	r.frames(2)
	var seen []string
	step := func(it backend.ChatItem, what string) {
		t.Helper()
		u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: it})
		for range 20 {
			r.now = r.now.Add(frameStep)
			r.frame()
			for _, label := range seen {
				if !r.shows(label) {
					t.Fatalf("%s: %q left the group:\n%s", what, label, r.drawn())
				}
			}
		}
		if name := it.Tool.Name; !slices.Contains(seen, name) {
			if !r.shows(name) {
				t.Fatalf("%s: %q never came into view:\n%s", what, name, r.drawn())
			}
			seen = append(seen, name)
		}
	}
	for i := range callRun {
		step(runCall(i, api.ToolRunning), callRun[i].name+" running")
		step(runCall(i, api.ToolDone), callRun[i].name+" ended")
	}
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "a1", Kind: api.ItemAgent, Turn: 1, At: novDay(18, 17, 59), Text: "All green."}})
	r.now = r.now.Add(frameStep)
	r.frame()
	if !r.shows("Edit") {
		t.Error("the group shut at once when text came, with no motion")
	}
	r.play(15) // past the fold's 200ms
	for _, label := range seen {
		if r.shows(label) {
			t.Errorf("%q still stands under the group once text came:\n%s", label, r.drawn())
		}
	}
}

// Folding the group by hand in the middle of a run keeps it folded until the
// run is over, a call coming in included; it stays shut after.
func TestAGroupFoldedByHandStaysFoldedForTheRun(t *testing.T) {
	u, r := lifeFrames(t, []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"},
		runCall(0, api.ToolRunning),
	}, true)
	r.frames(2)
	if !r.shows("Edit") {
		t.Fatalf("the run's call is not in view:\n%s", r.drawn())
	}
	r.queueClick(r.at(t, i18n.T("chat.view.toolCalls", i18n.Args{"count": 1})))
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
