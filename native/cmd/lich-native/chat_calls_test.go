package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

func toolItem(id, name, title, state string) backend.ChatItem {
	return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: name, Title: title, State: state}}
}

// Every provider's names for the same deed count as one kind: Claude Code's,
// Codex's, the kinds ACP sends, Antigravity's and Kiro CLI's.
func TestAGroupSaysWhatItsCallsDidOnEveryProvider(t *testing.T) {
	for provider, names := range map[string][]string{
		"claude":      {"Bash", "Read", "Grep", "WebFetch", "mcp__playwright__browser_click"},
		"codex":       {"Bash", "view_image", "web_search", "fetch", "mcp__gh__search"},
		"acp":         {"execute", "read", "search", "fetch", "switch_mode2"},
		"antigravity": {"run_command", "view_file", "list_dir", "fetch", "call_mcp_tool"},
		"kiro":        {"execute_bash", "fs_read", "grep", "webfetch", "use_aws"},
	} {
		var tools []backend.ChatItem
		for i, n := range names {
			tools = append(tools, toolItem(string(rune('a'+i)), n, "x", api.ToolDone))
		}
		tools = append(tools, toolItem("z", names[0], "x", api.ToolFailed))
		label, failed := callSummary(tools, func(backend.ChatItem) bool { return false })
		want := "Ran 2 commands, read 1 file, searched 1 time, fetched 1 page, used 1 other tool"
		if label != want || failed != "1 failed" {
			t.Errorf("%s: label %q · %q, want %q · %q", provider, label, failed, want, "1 failed")
		}
	}
}

func TestACallsVerbIsItsKindsOrItsOwnName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		running bool
		want    string
	}{
		{"Bash", false, "Ran"},
		{"Bash", true, "Running"},
		{"view_file", false, "Read"},
		{"apply_patch", false, "Edited"},
		{"mcp__playwright__browser_open", false, "browser_open"},
		{"TodoWrite", false, "TodoWrite"},
	} {
		if got := callVerb(&backend.ChatToolCall{Name: tc.name}, tc.running); got != tc.want {
			t.Errorf("callVerb(%s, running %v) = %q, want %q", tc.name, tc.running, got, tc.want)
		}
	}
}

// One call alone is its own row, naming what it ran with no fold to open;
// several fold under what they did, without the agent's avatar.
func TestACallAloneShowsItsTargetAndAGroupSaysWhatItDid(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"},
		toolItem("k1", "Bash", "git fetch -q origin", api.ToolDone),
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "fetched"},
		toolItem("k2", "Bash", "go build ./...", api.ToolDone),
		toolItem("k3", "Read", "smoke/shot.png", api.ToolDone),
		toolItem("k4", "Bash", "gh release create", api.ToolFailed),
	})
	r.settle(t)
	writeShot(t, r, "calls-settled.png")
	for _, s := range []string{"Ran", "git fetch -q origin", "Ran 2 commands, read 1 file", "· 1 failed"} {
		if !r.shows(s) {
			t.Errorf("the chat lacks %q:\n%s", s, r.drawn())
		}
	}
	if r.shows("tool call") || r.shows("go build ./...") {
		t.Errorf("the group counts tool calls or shows its calls before a click:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Ran 2 commands, read 1 file"))
	r.settle(t)
	writeShot(t, r, "calls-open.png")
	if !r.shows("go build ./...") || !r.shows("smoke/shot.png") {
		t.Errorf("the open group lacks its calls:\n%s", r.drawn())
	}
}

// While a call runs, the group's line is that call.
func TestARunningGroupNamesTheCallRunning(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	at := r.now.Add(-3 * time.Second).UnixMilli()
	run := toolItem("k2", "Bash", "go test ./cmd/lich-native", api.ToolRunning)
	run.At = at
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go", At: at},
		toolItem("k1", "Read", "go.mod", api.ToolDone),
		run,
	})
	r.frame()
	writeShot(t, r, "calls-live.png")
	if !r.shows("Running go test ./cmd/lich-native") || !r.shows("· 2 tool calls") {
		t.Errorf("the live group does not name the call running:\n%s", r.drawn())
	}
}
