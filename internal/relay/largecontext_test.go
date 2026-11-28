package relay

import (
	"context"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
)

// ContextTokens answers with what holdContext recorded; a session with nothing
// recorded is one whose provider reports no context.
func (f *fakeTerminal) ContextTokens(id string) (int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tokens, ok := f.contexts[id]
	return tokens, ok
}

func (f *fakeTerminal) holdContext(id string, tokens int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.contexts == nil {
		f.contexts = map[string]int{}
	}
	f.contexts[id] = tokens
}

// A task sent to a session whose context is past largeContextTokens is
// delivered as any other, and the result tells the sender how big that context
// is. Under the line, or where the provider reports no context, nothing is
// added.
func TestASendToALargeContextSaysHowLargeAndStillDelivers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		kind   string
		tokens int
		known  bool
		want   int
	}{
		{"Claude Code over the line", providers.Claude, 451_000, true, 451_000},
		{"Codex over the line", providers.Codex, 230_000, true, 230_000},
		{"Claude Code at the line", providers.Claude, largeContextTokens, true, 0},
		{"Codex under the line", providers.Codex, 120_000, true, 0},
		{"ACP with no context reported", providers.OpenCode, 0, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			term := newFakeTerminal("s1", "s2")
			if tc.known {
				term.holdContext("s2", tc.tokens)
			}
			roster := fakeSessions{projects: []store.Project{{ID: "p1", Name: "lich", Path: "/src/lich",
				Sessions: []store.Session{
					{ID: "s1", Label: "sender", Kind: providers.Claude},
					{ID: "s2", Label: "worker", Kind: tc.kind},
				}}}}
			svc := newRelay(roster, term, nil)

			got, err := svc.Send(context.Background(), SendOptions{From: "s1", Target: "worker", Prompt: "fix the flake", WaitSeconds: 1})
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if !strings.Contains(term.written("s2"), "fix the flake") {
				t.Errorf("the task never reached the target: %q", term.written("s2"))
			}
			if got.TargetContext != tc.want {
				t.Errorf("TargetContext = %d, want %d", got.TargetContext, tc.want)
			}
		})
	}
}
