package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// A provider subagent call lich opened as a card is drawn as opened, its
// label a click to the card, never as the failed call the provider reports.
func TestAVetoedCallIsDrawnAsOpened(t *testing.T) {
	failed := "PreToolUse:Agent hook error: lich opened this task as the session \"Fix #412\" (ticket 4b9bc94e)."
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "Fix #412 in a subagent."},
		{ID: "toolu_1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Agent", Title: "Fix the handshake race", State: api.ToolFailed, Output: failed}},
		{ID: "toolu_2", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Read", Title: "internal/relay/conn.go", State: api.ToolFailed, Output: "no such file"}},
	}
	u, r := envelopeRig(t, items)
	u.model.vetoes.add(api.SubagentVeto{SessionID: "s1", ToolUseID: "toolu_1", Sessions: []api.SubagentOpened{{ID: "s2", Label: "Fix #412", Ticket: "4b9bc94e"}}})
	r.frame()
	if r.shows("· 2 failed") || !r.shows("· 1 failed") {
		t.Errorf("the group counts the vetoed call as failed:\n%s", r.drawn())
	}
	r.clickLabel(t, "Read 1 file, used 1 other tool")
	for range 30 {
		r.now = r.now.Add(20 * time.Millisecond)
		r.frame()
	}
	if !r.shows("opened as Fix #412") {
		t.Fatalf("the vetoed call is not drawn as opened:\n%s", r.drawn())
	}
	if strings.Contains(r.drawn(), "hook error") {
		t.Errorf("the vetoed call still reads as failed:\n%s", r.drawn())
	}
	if !r.shows("no such file") {
		t.Errorf("a call lich did not veto lost its own failure:\n%s", r.drawn())
	}
	writeShot(t, r, "vetoed-call.png")
	r.clickLabel(t, "opened as Fix #412")
	if sv, ok := u.model.activeSession(); !ok || sv.ID != "s2" {
		t.Errorf("active session = %q, want the card s2", sv.ID)
	}
}
