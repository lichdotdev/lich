package chat

import (
	"encoding/json"
	"testing"
)

// A lich tool's call names the session or service it acted on, which the
// window turns into a link to it.
func TestLichToolTitle(t *testing.T) {
	titles := map[string]func(name string, args json.RawMessage) string{
		"claude": func(name string, args json.RawMessage) string { return claudeTool(name, args).Title },
		"codex":  func(_ string, args json.RawMessage) string { return argsTitle(args) },
	}
	cases := []struct {
		tool, args, want string
	}{
		{"mcp__lich__send_to_session", `{"session":"fix-relay","prompt":"go"}`, "fix-relay"},
		{"mcp__lich__close_session", `{"session":"s7","worktree":"keep"}`, "s7"},
		{"mcp__lich__restart_service", `{"service":"api dev"}`, "api dev"},
		{"mcp__lich__show_code", `{"path":"a.go","line":3}`, "a.go"},
	}
	for provider, title := range titles {
		for _, c := range cases {
			if got := title(c.tool, json.RawMessage(c.args)); got != c.want {
				t.Errorf("%s %s: title %q, want %q", provider, c.tool, got, c.want)
			}
		}
	}
}
