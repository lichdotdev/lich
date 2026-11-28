// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"testing"

	"github.com/lichdotdev/lich/internal/events"
)

// ContextTokens is the footer's context readout, tokens only: what a Claude
// transcript or a Codex rollout records, and nothing for a provider that
// records no window, which a relay reads as no context to speak of.
func TestContextTokensReadsTheFootersContext(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		plant  func(t *testing.T)
		tokens int
		ok     bool
	}{
		{"Claude Code", "uuid-claude", func(t *testing.T) {
			writeTranscript(t, "uuid-claude", assistantLine(modelOpus, 2, 450_998, 0)+"\n")
		}, 451_000, true},
		{"Codex", "uuid-codex", func(t *testing.T) {
			writeCodexTranscript(t, "uuid-codex",
				`{"type":"turn_context","payload":{"model":"gpt-5.6-sol"}}`+"\n"+
					`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"total_tokens":65111},"model_context_window":258400}}}`+"\n")
		}, 65111, true},
		{"opencode", "ses_one", func(t *testing.T) {
			writeOpenCodeDB(t, sessionRow{id: "ses_one", cost: 0.021})
		}, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.plant(t)
			svc := New(stubBins{providerSession: tc.id}, nil, events.New())

			tokens, ok := svc.ContextTokens("s1")
			if tokens != tc.tokens || ok != tc.ok {
				t.Errorf("ContextTokens = %d, %v, want %d, %v", tokens, ok, tc.tokens, tc.ok)
			}
		})
	}
}
