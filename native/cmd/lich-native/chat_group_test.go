package main

import (
	"slices"
	"strconv"
	"testing"
	"time"

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

// An open group of more than chatCallsShown calls shows its last ones under a
// line that counts the rest; a click on it shows them all, and Show less
// hides them again.
func TestAnOpenGroupShowsItsLastCallsFirst(t *testing.T) {
	items := []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, At: novDay(18, 17, 58), Text: "go"}}
	for i := range 8 {
		items = append(items, backend.ChatItem{ID: "c" + strconv.Itoa(i), Kind: api.ItemTool, Turn: 1, At: novDay(18, 17, 59),
			Tool: &backend.ChatToolCall{Name: "Grep", Title: "call-" + strconv.Itoa(i), State: api.ToolDone}})
	}
	u, r := lifeFrames(t, items, false)
	r.play(15)
	u.chats.pane("s1").fold("work:c0").open = true
	r.play(15)
	shown := func(from int) {
		t.Helper()
		for i := range 8 {
			if got := r.shows("call-" + strconv.Itoa(i)); got != (i >= from) {
				t.Errorf("call %d drawn %v with calls from %d shown:\n%s", i, got, from, r.drawn())
			}
		}
	}
	shown(3)
	writeShot(t, r, "group-earlier.png")
	more := i18n.T("chat.view.earlierCalls", i18n.Args{"count": 3})
	r.queueClick(r.at(t, more))
	r.play(15)
	shown(0)
	r.queueClick(r.at(t, i18n.T("chat.view.shellLess")))
	r.play(15)
	shown(3)
}

// A turn's calls fold into one line per run, a run ending where the agent
// speaks, each line where its run began; the turn's end counts them all.
func TestATurnFoldsEachRunOfCallsWhereItRan(t *testing.T) {
	call := func(id string) backend.ChatItem {
		return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Grep", State: api.ToolDone}}
	}
	items := []backend.ChatItem{
		{ID: "u", Kind: api.ItemUser, Turn: 1, Text: "go"},
		call("k1"), call("k2"),
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "reading"},
		{ID: "th", Kind: api.ItemThought, Turn: 1, Text: "hm"},
		call("k3"),
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, Text: "done"},
	}
	var got []string
	done := 0
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		switch r.kind {
		case chatRowWork:
			got = append(got, chatRowKey(r)+"="+strconv.Itoa(len(r.tools)))
		case chatRowItem:
			got = append(got, r.item.ID)
		case chatRowDone:
			done = len(r.tools)
		}
	}
	want := []string{"u", "work:k1=2", "a1", "th", "work:k3=1", "a2"}
	if !slices.Equal(got, want) || done != 3 {
		t.Errorf("rows %v and %d calls at the end, want %v and 3", got, done, want)
	}
	if rows := chatRows(items, claudeKind, false, time.Time{}); !rows[1].avatar || rows[4].avatar {
		t.Error("the avatar is not on the turn's first line alone")
	}
}
