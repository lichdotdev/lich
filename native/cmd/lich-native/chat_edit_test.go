package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// A call that changed a file stands in the chat as a row of its own, whatever
// its provider calls it; the calls that did not stay in the turn's fold.
func TestAnEditStandsOutsideTheFold(t *testing.T) {
	call := func(id, name, diff string) backend.ChatItem {
		return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: name, State: api.ToolDone, Diff: diff}}
	}
	items := []backend.ChatItem{
		{ID: "u", Kind: api.ItemUser, Turn: 1, Text: "go"},
		call("r", "Read", ""),
		call("w", "Write", "+a"),
		call("c", "fileChange", "+b"),
		call("e", "edit", "-c"),
		call("g", "Grep", ""),
	}
	var edits []string
	folded := 0
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		switch r.kind {
		case chatRowEdit:
			edits = append(edits, r.item.ID)
		case chatRowWork:
			folded += len(r.tools)
		}
	}
	if len(edits) != 3 || edits[0] != "w" || edits[1] != "c" || edits[2] != "e" || folded != 2 {
		t.Errorf("edit rows %v and %d folded calls, want [w c e] and 2", edits, folded)
	}
}
