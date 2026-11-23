package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// Texts the agent writes one after another in a turn read as one answer: only
// the last keeps the time and copy line under it, and its copy takes them all.
func TestAnAnswersTextsInARowShareOneCopyLine(t *testing.T) {
	now := time.Unix(1000, 0)
	items := []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "reading the relay"},
		{ID: "t1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolDone}},
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, Text: "relay fixed"},
		{ID: "a3", Kind: api.ItemAgent, Turn: 1, Text: "gate green"},
		{ID: "u2", Kind: api.ItemUser, Turn: 2, Text: "next"},
		{ID: "a4", Kind: api.ItemAgent, Turn: 2, Text: "on it"},
	}
	rows := chatRows(items, claudeKind, false, now)
	byID := map[string]chatRow{}
	for _, r := range rows {
		if r.kind == chatRowItem {
			byID[r.item.ID] = r
		}
	}
	if byID["a2"].more == false || byID["a3"].more || byID["a1"].more || byID["a4"].more {
		t.Errorf("more: a1=%v a2=%v a3=%v a4=%v, want only a2 (followed by a3)", byID["a1"].more, byID["a2"].more, byID["a3"].more, byID["a4"].more)
	}
	if got := byID["a3"].runText; got != "relay fixed\n\ngate green" {
		t.Errorf("a3's copy takes %q, want both texts of the run", got)
	}
	if byID["a1"].runText != "" || byID["a4"].runText != "" {
		t.Error("a text alone carries a run's copy")
	}
}
