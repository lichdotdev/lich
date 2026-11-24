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

const editDiff = "--- a/app/page.tsx\n+++ b/app/page.tsx\n@@ -1,3 +1,4 @@\n import x from 'y'\n+const a = 1\n-const c = 3\n+const b = 2\n"

// An edit is one line naming the file and its counts; a click shows its
// lines, without the hunk's "@@" header, and a second one folds them.
func TestAnEditIsALineThatOpensItsDiff(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"},
		{ID: "e1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Edit", Title: "/repo/app/page.tsx", State: api.ToolDone, Diff: editDiff}},
	})
	r.settle(t)
	writeShot(t, r, "edit-shut.png")
	for _, s := range []string{"app/", "page.tsx", "+2", "-1"} {
		if !r.shows(s) {
			t.Errorf("the edit line lacks %q:\n%s", s, r.drawn())
		}
	}
	if r.shows("+const a = 1") || r.shows("Edit") {
		t.Fatalf("the edit shows its diff or its tool's name before a click:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "page.tsx"))
	r.settle(t)
	writeShot(t, r, "edit-open.png")
	if !r.shows("+const a = 1") || r.shows("@@ -1,3 +1,4 @@") {
		t.Errorf("the open edit lacks its lines or shows the hunk header:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "page.tsx"))
	r.settle(t)
	if r.shows("+const a = 1") {
		t.Errorf("a second click left the diff open:\n%s", r.drawn())
	}
}

func TestEditPath(t *testing.T) {
	for name, tc := range map[string]struct {
		diff, title, checkout, want string
	}{
		"the diff's new side":       {"--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n", "", "", "x.go"},
		"a file deleted":            {"--- a/gone.go\n+++ /dev/null\n@@ -1 +0,0 @@\n", "", "", "gone.go"},
		"absolute, in the checkout": {"--- /repo/a/b.go\n+++ /repo/a/b.go\n", "", "/repo", "a/b.go"},
		"absolute, outside":         {"", "/etc/hosts", "/repo", "/etc/hosts"},
		"no diff, the target":       {"", "notes.md", "/repo", "notes.md"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := editPath(&backend.ChatToolCall{Diff: tc.diff, Title: tc.title}, tc.checkout); got != tc.want {
				t.Errorf("editPath = %q, want %q", got, tc.want)
			}
		})
	}
}
