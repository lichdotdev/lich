//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func TestParseCommandNeedsExactlyTheCommandsTags(t *testing.T) {
	for name, c := range map[string]struct {
		text, want string
		ok         bool
	}{
		"name, message, args":    {"<command-name>/model</command-name><command-message>model</command-message><command-args>haiku</command-args>", "/model haiku", true},
		"on lines, indented":     {"<command-name>/model</command-name>\n  <command-message>model</command-message>\n  <command-args>haiku</command-args>\n", "/model haiku", true},
		"message first":          {"<command-message>clear</command-message>\n<command-name>/clear</command-name>\n<command-args></command-args>", "/clear", true},
		"no args":                {"<command-name>/compact</command-name><command-message>compact</command-message>", "/compact", true},
		"no name":                {"<command-message>model</command-message><command-args>haiku</command-args>", "", false},
		"text around the tags":   {"see <command-name>/model</command-name> now", "", false},
		"an unknown tag with it": {"<command-name>/model</command-name><command-extra>x</command-extra>", "", false},
		"plain text":             {"/model haiku", "", false},
	} {
		got, ok := parseCommand(c.text)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: %q, %v; want %q, %v", name, got, ok, c.want, c.ok)
		}
	}
}

func TestParseStdoutNeedsOnlyTheStdoutTag(t *testing.T) {
	for text, want := range map[string]string{
		"<local-command-stdout>Set model to haiku</local-command-stdout>":         "Set model to haiku",
		"\n<local-command-stdout>\nline one\nline two\n</local-command-stdout>\n": "line one\nline two",
		"<local-command-stdout></local-command-stdout>":                           "",
	} {
		if got, ok := parseStdout(text); !ok || got != want {
			t.Errorf("parseStdout(%q) = %q, %v; want %q", text, got, ok, want)
		}
	}
	for _, text := range []string{"<local-command-stderr>no</local-command-stderr>", "x <local-command-stdout>y</local-command-stdout>", "plain"} {
		if _, ok := parseStdout(text); ok {
			t.Errorf("parseStdout(%q) took it", text)
		}
	}
}

// texts are the texts of the items of kind, in order.
func (r *rig) texts(kind string) []string {
	var out []string
	for _, it := range r.history() {
		if it.Kind == kind {
			out = append(out, it.Text)
		}
	}
	return out
}

// TestLocalCommandsOfTheTranscriptAreNoticesAndTheirSyntheticAnswerIsDropped:
// the commands a person ran in the terminal come back as one muted line, and
// the answer Claude Code writes after them is not the agent's.
func TestLocalCommandsOfTheTranscriptAreNoticesAndTheirSyntheticAnswerIsDropped(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"user","message":{"content":"asked"}}`,
		`{"type":"assistant","message":{"id":"h1","content":[{"type":"text","text":"answered"}]}}`,
		`{"type":"user","message":{"content":"<command-name>/model</command-name>\n            <command-message>model</command-message>\n            <command-args>haiku</command-args>"}}`,
		`{"type":"user","message":{"content":"<local-command-stdout>Set model to haiku (claude-haiku-5-5)</local-command-stdout>"}}`,
		`{"type":"assistant","message":{"id":"h2","content":[{"type":"text","text":"No response requested."}]}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"<command-name>/clear</command-name><command-message>clear</command-message><command-args></command-args>"}]}}`,
		`{"type":"user","message":{"content":"<local-command-stderr>oops</local-command-stderr>"}}`,
		`{"type":"assistant","message":{"id":"h3","content":[{"type":"text","text":"No response requested. Really."}]}}`,
	)
	if _, err := r.term.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: r.home, Kind: providers.Claude, Resume: "conv-1", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if got, want := strings.Join(r.texts(api.ItemNotice), "|"), "/model haiku · Set model to haiku (claude-haiku-5-5)|/clear"; got != want {
		t.Errorf("notices %q, want %q", got, want)
	}
	// A tag the chat does not know stays what it was: a message of the person.
	if got, want := strings.Join(r.texts(api.ItemUser), "|"), "asked|<local-command-stderr>oops</local-command-stderr>"; got != want {
		t.Errorf("messages %q, want %q", got, want)
	}
	if got, want := strings.Join(r.texts(api.ItemAgent), "|"), "answered|No response requested. Really."; got != want {
		t.Errorf("answers %q, want %q (only the exact text is dropped)", got, want)
	}
}

// TestLiveLocalCommandsAndTheSyntheticAnswerNeverReachTheWindow: the same on
// the stream, where the synthetic answer arrives a few characters at a time
// and must not flash on the way.
func TestLiveLocalCommandsAndTheSyntheticAnswerNeverReachTheWindow(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("local")
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, "No way")
	if got, want := strings.Join(r.texts(api.ItemNotice), "|"), "/model haiku · Set model to haiku (claude-haiku-5-5)"; got != want {
		t.Errorf("notices %q, want %q", got, want)
	}
	if got, want := strings.Join(r.texts(api.ItemAgent), "|"), "No way"; got != want {
		t.Errorf("answers %q, want %q", got, want)
	}
	for _, e := range r.snapshot() {
		if u, ok := e.data.(api.ChatItemUpdate); ok && strings.Contains(u.Item.Text, "No resp") && u.Item.Kind == api.ItemAgent {
			t.Errorf("the window was shown %q", u.Item.Text)
		}
		if u, ok := e.data.(api.ChatItemUpdate); ok && u.Item.Kind == api.ItemUser && strings.Contains(u.Item.Text, "<command") {
			t.Errorf("the window was shown the command as a message: %q", u.Item.Text)
		}
	}
}
