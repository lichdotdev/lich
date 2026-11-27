package main

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// Each harness spells a lich tool its own way (toolline.go); every spelling
// wears the tool's family mark and speaks its verb, and another server's tool
// of the same name stays a plain MCP call.
func TestLichToolEverySpelling(t *testing.T) {
	for _, name := range []string{"mcp__lich__send_to_session", "mcp__lich_send_to_session", "lich_send_to_session"} {
		tool, call, ok := lichTool(name)
		if !ok || tool != "send_to_session" || !call.atSession {
			t.Errorf("%s: tool %q, at session %v, ok %v", name, tool, call.atSession, ok)
		}
		if toolIcon(name) != icLichSend {
			t.Errorf("%s does not wear the send mark", name)
		}
		if got := callVerb(&backend.ChatToolCall{Name: name}, false); got != "Sent to" {
			t.Errorf("%s: verb %q", name, got)
		}
		if got := callVerb(&backend.ChatToolCall{Name: name}, true); got != "Sending to" {
			t.Errorf("%s running: verb %q", name, got)
		}
	}
	if _, _, ok := lichTool("mcp__other__send_to_session"); ok {
		t.Error("another server's tool was taken for lich's")
	}
	if toolIcon("mcp__lich__browser_hover") != icLichBrowser {
		t.Error("a browser tool the table does not list lost its family mark")
	}
	if got := callVerb(&backend.ChatToolCall{Name: "mcp__lich__list_services"}, false); got != "list_services" {
		t.Errorf("a tool with no verb reads %q, want its name", got)
	}
}

// A group counts its lich calls as their own clause, apart from other tools.
func TestAGroupCountsItsLichCalls(t *testing.T) {
	tools := []backend.ChatItem{
		toolItem("a", "Bash", "ls", api.ToolDone),
		toolItem("b", "mcp__lich__send_to_session", "fix", api.ToolDone),
		toolItem("c", "lich_close_session", "fix", api.ToolDone),
		toolItem("d", "mcp__gh__search", "x", api.ToolDone),
	}
	if got, want := callSummaryLabel(tools), "Ran 1 command, made 2 lich calls, used 1 other tool"; got != want {
		t.Errorf("label %q, want %q", got, want)
	}
}

// A lich tool names a session by id or by the label on its card; a label two
// cards of the project share names neither.
func TestSessionCalled(t *testing.T) {
	m := &model{projects: []backend.Project{
		{ID: "p", Sessions: []backend.Session{{ID: "s1", Label: "fix"}, {ID: "s2", Label: "twin"}, {ID: "s3", Label: "twin"}}},
		{ID: "q", Sessions: []backend.Session{{ID: "s4", Label: "fix"}}},
	}}
	cases := []struct {
		project, target, id string
		ok                  bool
	}{
		{"p", "fix", "s1", true},
		{"q", "fix", "s4", true},
		{"q", "s1", "s1", true},
		{"p", "twin", "", false},
		{"p", "gone", "", false},
	}
	for _, c := range cases {
		id, _, ok := m.sessionCalled(c.project, c.target)
		if id != c.id || ok != c.ok {
			t.Errorf("%s in %s = %q %v, want %q %v", c.target, c.project, id, ok, c.id, c.ok)
		}
	}
}

// Only a lich call that reached an open session draws its pill: not a failed
// one, which says why in its row, nor one whose session is not open.
func TestLichOutcomeOnlyAtAnOpenSession(t *testing.T) {
	u := &window{model: &model{projects: []backend.Project{{ID: "p", Sessions: []backend.Session{{ID: "s1", Label: "fix"}}}}}}
	outcome := u.lichOutcome(&chatPane{}, sessionView{projectID: "p"})
	call := func(name, title, state string) backend.ChatItem {
		return backend.ChatItem{ID: "c", Tool: &backend.ChatToolCall{Name: name, Title: title, State: state}}
	}
	if outcome(call("mcp__lich__send_to_session", "fix", api.ToolDone)) == nil {
		t.Error("a call at an open session drew no pill")
	}
	for _, it := range []backend.ChatItem{
		call("mcp__lich__send_to_session", "fix", api.ToolFailed),
		call("mcp__lich__send_to_session", "gone", api.ToolDone),
		call("mcp__lich__restart_service", "fix", api.ToolDone),
		call("Bash", "fix", api.ToolDone),
	} {
		if outcome(it) != nil {
			t.Errorf("%s at %q (%s) drew a pill", it.Tool.Name, it.Tool.Title, it.Tool.State)
		}
	}
}

// TestLichCallShots writes a chat of lich calls, one per row and folded into
// a group, dark and light, where LICH_SHOT_DIR names a folder.
func TestLichCallShots(t *testing.T) {
	calls := []backend.ChatItem{
		toolItem("c1", "mcp__lich__open_session", "fix-412", api.ToolDone),
		toolItem("c3", "mcp__lich__wait_for_answer", "", api.ToolDone),
		toolItem("c4", "mcp__lich__restart_service", "api dev", api.ToolDone),
		toolItem("c2", "mcp__lich__send_to_session", "Fix #412", api.ToolDone),
		toolItem("c5", "mcp__lich__browser_open", "http://localhost:5173", api.ToolDone),
		toolItem("c6", "mcp__lich__show_code", "internal/chat/tool.go", api.ToolDone),
		toolItem("c7", "mcp__lich__close_session", "Fix #412", api.ToolFailed),
	}
	rows := []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "hand #412 to a worker"}}
	for i, c := range calls {
		rows = append(rows, backend.ChatItem{ID: "a" + c.ID, Kind: api.ItemAgent, Turn: 1, Text: "Step " + string(rune('1'+i)) + "."}, c)
	}
	group := append([]backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "hand #412 to a worker"}}, calls...)
	for _, light := range []bool{false, true} {
		scheme := map[bool]string{false: "dark", true: "light"}[light]
		writeShot(t, compactShot(t, rows, false, light), "lich-rows-"+scheme+".png")
		r := compactShot(t, group, false, light)
		r.click(t, r.at(t, callSummaryLabel(group)))
		r.frames(maxSettleFrames)
		writeShot(t, r, "lich-group-"+scheme+".png")
	}
}

func callSummaryLabel(items []backend.ChatItem) string {
	label, _ := callSummary(items, func(backend.ChatItem) bool { return false })
	return label
}
