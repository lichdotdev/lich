//go:build !windows

package chat

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// suggestions is every suggestion the window was sent, the clears included.
func (r *rig) suggestions() []string {
	var out []string
	for _, e := range r.snapshot() {
		if s, ok := e.data.(api.ChatSuggestion); ok && e.name == api.ChatSuggestionEvent.Name {
			out = append(out, s.Text)
		}
	}
	return out
}

// waitSuggestion waits for a suggestion with text.
func (r *rig) waitSuggestion(text string) {
	r.t.Helper()
	r.eventually("suggestion "+text, func() bool {
		return slices.Contains(r.suggestions(), text)
	})
}

// settled gives a suggestion that should not come the time one would.
func (r *rig) settled() { time.Sleep(300 * time.Millisecond) }

func (r *rig) noSuggestion() {
	r.t.Helper()
	r.settled()
	for _, s := range r.suggestions() {
		if s != "" {
			r.t.Fatalf("suggestions = %q, want none", r.suggestions())
		}
	}
}

func newClaudeChat(t *testing.T) *rig {
	t.Helper()
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	return r
}

func (r *rig) take(text string) {
	r.t.Helper()
	if _, err := r.svc.Take("s1", text); err != nil {
		r.t.Fatalf("Take: %v", err)
	}
}

const testEnvelope = `<lich-notification version="1" kind="notice">` + "\n<summary>done</summary>\n</lich-notification>"

// TestClaudeSuggestion: initialize asks for suggestions, the prompt_suggestion
// written after the turn's result reaches the window trimmed, and the next
// turn clears it.
func TestClaudeSuggestion(t *testing.T) {
	r := newClaudeChat(t)
	if log := r.fakeLog(); !strings.Contains(log, `"promptSuggestions":true`) {
		t.Errorf("initialize did not ask for suggestions: %q", log)
	}
	r.prompt("suggest")
	r.waitSuggestion("commit this")
	r.prompt("hi")
	r.turnEnd(1)
	if got := r.suggestions(); len(got) != 3 || got[0] != "" || got[1] != "commit this" || got[2] != "" {
		t.Errorf("suggestions = %q, want a clear, the suggestion, a clear", got)
	}
	if strings.Contains(r.fakeLog(), "set_prompt_suggestions_paused") {
		t.Error("a session nobody opened was paused")
	}
}

// TestClaudeNoSuggestionAfterEnvelope: a turn lich's relay opened gets none.
func TestClaudeNoSuggestionAfterEnvelope(t *testing.T) {
	r := newClaudeChat(t)
	r.take(testEnvelope)
	r.turnEnd(0)
	r.noSuggestion()
}

// TestClaudeWorkerPausedUntilTyped: a session another session opened starts
// paused, shows nothing, and the person's first message lifts the pause.
func TestClaudeWorkerPausedUntilTyped(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.db.SetSessionSubagentDepth("s1", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if log := r.fakeLog(); !strings.Contains(log, `"paused":true,"subtype":"set_prompt_suggestions_paused"`) {
		t.Errorf("the worker was not paused: %q", log)
	}
	r.take("suggest")
	r.turnEnd(0)
	r.noSuggestion()
	r.prompt("suggest")
	r.waitSuggestion("commit this")
	if !strings.Contains(r.fakeLog(), `"paused":false`) {
		t.Error("the person's message did not lift the pause")
	}
}

// TestCodexSuggestionFork: the suggestion comes from an ephemeral, read-only
// fork with no shell, asked at low effort on the session's model; its reply is
// cleaned, and nothing the fork says reaches the chat, its options or its turn.
func TestCodexSuggestionFork(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("suggest")
	r.waitSuggestion("run the tests")
	if slices.Contains(r.suggestions(), "stray") {
		t.Errorf("another thread's answer became the suggestion: %q", r.suggestions())
	}
	var fork, forkTurn string
	for line := range strings.Lines(r.fakeLog()) {
		switch {
		case strings.Contains(line, `"thread/fork"`):
			fork = line
		case strings.Contains(line, `"turn/start"`) && strings.Contains(line, "codex-fork-"):
			forkTurn = line
		}
	}
	for _, want := range []string{`"ephemeral":true`, `"excludeTurns":true`, `"sandbox":"read-only"`, `"approvalPolicy":"never"`,
		`"features.shell_tool":false`, `"features.unified_exec":false`, `"model":"gpt-a"`, `"threadId":"codex-new"`} {
		if !strings.Contains(fork, want) {
			t.Errorf("thread/fork = %q, want %s", fork, want)
		}
	}
	if !strings.Contains(forkTurn, `"effort":"low"`) || !strings.Contains(forkTurn, `"model":"gpt-a"`) || !strings.Contains(forkTurn, "SUGGESTION MODE") {
		t.Errorf("fork turn/start = %q", forkTurn)
	}
	r.eventually("the fork let go", func() bool { return r.unsubscribed("codex-fork-suggest") })
	for _, it := range r.history() {
		if strings.Contains(it.Text, "run the tests") || strings.Contains(it.Text, "SUGGESTION") {
			t.Errorf("the fork reached the chat: %+v", it)
		}
	}
	if turns := len(r.turnStarts()); turns != 2 {
		t.Errorf("%d turn/starts, want the person's and the fork's", turns)
	}
	if ends := r.turnEnds(); ends != 1 {
		t.Errorf("%d turn ends, want 1", ends)
	}
	d := r.svc.running("s1").drv.(*codexDriver)
	d.mu.Lock()
	effort, mode, turn := d.run.effort, d.run.mode, d.turn
	d.mu.Unlock()
	if effort != "medium" || mode != codexWorkspace || turn == "fork-turn" {
		t.Errorf("the fork moved the session: effort %q, mode %q, turn %q", effort, mode, turn)
	}
}

// unsubscribed is whether lich let go of thread, which Codex then unloads.
func (r *rig) unsubscribed(thread string) bool {
	for line := range strings.Lines(r.fakeLog()) {
		if strings.Contains(line, `"thread/unsubscribe"`) && strings.Contains(line, `"threadId":"`+thread+`"`) {
			return true
		}
	}
	return false
}

// turnEnds counts the turns the window was told ended.
func (r *rig) turnEnds() int {
	n := 0
	for _, e := range r.snapshot() {
		if t, ok := e.data.(api.ChatTurn); ok && !t.Running {
			n++
		}
	}
	return n
}

// TestCodexForkToolMakesNoSuggestion: a fork that calls a tool says nothing.
func TestCodexForkToolMakesNoSuggestion(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("suggest-tool")
	r.eventually("the fork unloaded", func() bool { return strings.Contains(r.fakeLog(), `"thread/unsubscribe"`) })
	r.noSuggestion()
}

// TestCodexNewTurnDropsFork: a turn the person begins before the fork answers
// interrupts it, and its late reply is never shown.
func TestCodexNewTurnDropsFork(t *testing.T) {
	r := newCodexChat(t)
	r.prompt("suggest-slow")
	r.eventually("the fork's turn", func() bool {
		return strings.Contains(r.fakeLog(), "codex-fork-suggest-slow") && len(r.turnStarts()) == 2
	})
	r.prompt("hi")
	r.turnEnd(1)
	r.eventually("the fork interrupted", func() bool {
		return strings.Contains(r.fakeLog(), `"turn/interrupt"`) && r.unsubscribed("codex-fork-suggest-slow")
	})
	r.noSuggestion()
}

// TestCodexNoForkAfterEnvelope: a turn lich's relay opened is not forked.
func TestCodexNoForkAfterEnvelope(t *testing.T) {
	r := newCodexChat(t)
	r.take(testEnvelope)
	r.turnEnd(0)
	r.settled()
	if strings.Contains(r.fakeLog(), `"thread/fork"`) {
		t.Error("an envelope's turn was forked")
	}
}

// TestCodexWorkerNotForked: a worker's turns are not forked until the person
// writes in it.
func TestCodexWorkerNotForked(t *testing.T) {
	r := newRig(t, providers.Codex)
	if err := r.db.SetSessionSubagentDepth("s1", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.take("suggest")
	r.turnEnd(0)
	r.settled()
	if strings.Contains(r.fakeLog(), `"thread/fork"`) {
		t.Error("a worker's turn was forked")
	}
	r.prompt("suggest")
	r.waitSuggestion("run the tests")
}

func TestCleanSuggestion(t *testing.T) {
	for in, want := range map[string]string{
		"  run the tests \n":                       "run the tests",
		"<suggestion>commit this</suggestion>":     "commit this",
		"<RESPONSE> push it </RESPONSE>":           "push it",
		"Suggestion: run the tests":                "run the tests",
		"<answer>Answer: yes</answer>":             "yes",
		"result:go ahead":                          "go ahead",
		"":                                         "",
		"<output></output>":                        "",
		"<suggestion>half":                         "<suggestion>half",
		"Suggestions are off":                      "Suggestions are off",
		"<result>Response: try it out</result>\n ": "try it out",
	} {
		if got := cleanSuggestion(in); got != want {
			t.Errorf("cleanSuggestion(%q) = %q, want %q", in, got, want)
		}
	}
}

func newACPChat(t *testing.T) *rig {
	t.Helper()
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	return r
}

// TestACPAgentSuggestion: lich asks an ACP agent for its own suggestions, and
// the first of a prompt_suggestions_update heard during the turn shows once
// the turn ends.
func TestACPAgentSuggestion(t *testing.T) {
	r := newACPChat(t)
	if log := r.fakeLog(); !strings.Contains(log, `"promptSuggestions":{}`) {
		t.Errorf("initialize did not ask for suggestions: %q", log)
	}
	r.prompt("suggest")
	r.waitSuggestion("run the tests")
}

// TestACPNoSuggestionAfterEnvelope: the agent's suggestion after a turn lich's
// relay opened is dropped, as every provider's is.
func TestACPNoSuggestionAfterEnvelope(t *testing.T) {
	r := newACPChat(t)
	r.take(testEnvelope)
	r.turnEnd(0)
	r.noSuggestion()
}
