//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func TestOnlyTheExactInterruptMarkersAreMarkers(t *testing.T) {
	for text, want := range map[string]bool{
		"[Request interrupted by user]":                 true,
		"[Request interrupted by user for tool use]":    true,
		"[Request interrupted by user for tool use] ok": false,
		"what does [Request interrupted by user] mean":  false,
		"[request interrupted by user]":                 false,
		"":                                              false,
	} {
		if got := isInterruptMarker(text); got != want {
			t.Errorf("isInterruptMarker(%q) = %v, want %v", text, got, want)
		}
	}
}

// userTexts are the person's messages in the chat.
func (r *rig) userTexts() []string {
	var out []string
	for _, it := range r.history() {
		if it.Kind == api.ItemUser {
			out = append(out, it.Text)
		}
	}
	return out
}

// TestInterruptMarkersAreNotMessagesOfThePerson: Claude Code writes them into
// the user's turn, as text or as a block, when a turn or a tool call is
// interrupted; neither the live stream nor the transcript turns one into a
// message.
func TestInterruptMarkersAreNotMessagesOfThePerson(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"user","message":{"content":"asked"}}`,
		`{"type":"user","message":{"content":"[Request interrupted by user]"}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"[Request interrupted by user]"}]}}`,
		`{"type":"user","message":{"content":"[Request interrupted by user for tool use]"}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}`,
		`{"type":"user","message":{"content":"what does [Request interrupted by user] mean?"}}`,
	)
	if _, err := r.term.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: r.home, Kind: providers.Claude, Resume: "conv-1", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if got, want := strings.Join(r.userTexts(), "|"), "asked|what does [Request interrupted by user] mean?"; got != want {
		t.Errorf("history holds the messages %q, want %q", got, want)
	}

	r.prompt("interrupted")
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, "stopped")
	if got, want := strings.Join(r.userTexts(), "|"), "asked|what does [Request interrupted by user] mean?|interrupted"; got != want {
		t.Errorf("after a live turn the messages are %q, want %q", got, want)
	}
}
