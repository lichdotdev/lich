package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/godemirror"
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

// An edit is one line naming the file and its counts; a click opens its diff
// in the Review panel's code view under it, and a second one folds it.
func TestAnEditIsALineThatOpensItsDiff(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.theme.Code = godemirror.DarkTheme
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
	if r.shows("Edit") {
		t.Fatalf("the edit shows its tool's name:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "page.tsx"))
	r.settle(t)
	writeShot(t, r, "edit-open.png")
	f := u.chats.pane("s1").fold("edit:e1")
	if !f.open || f.diff == nil || f.diff.fail || len(f.diff.files) != 1 || f.diff.files[0].rows != 4 {
		t.Fatalf("the open edit's diff = %+v, want one file of 4 rows in the code view", f.diff)
	}
	if r.shows("+const a = 1") {
		t.Errorf("the open edit drew its lines as text, not in the code view:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "page.tsx"))
	r.settle(t)
	if f.open {
		t.Errorf("a second click left the diff open")
	}
}

// A long edit shows its first chatDiffShown rows and the way to the rest.
func TestALongEditShowsItsFirstRowsAndTheWayToTheRest(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.theme.Code = godemirror.DarkTheme
	var b strings.Builder
	b.WriteString("--- /dev/null\n+++ b/a.go\n@@ -0,0 +1,50 @@\n")
	for i := range 50 {
		fmt.Fprintf(&b, "+line %d\n", i)
	}
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"},
		{ID: "e1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Write", Title: "/repo/a.go", State: api.ToolDone, Diff: b.String()}},
	})
	r.settle(t)
	r.click(t, r.at(t, "a.go"))
	r.settle(t)
	writeShot(t, r, "edit-long.png")
	if !r.shows("Show 30 more lines") {
		t.Fatalf("the long edit lacks the way to its last 30 rows:\n%s", r.drawn())
	}
	r.click(t, r.at(t, "Show 30 more lines"))
	r.settle(t)
	if r.shows("Show 30 more lines") || !u.chats.pane("s1").fold("edit:e1").diff.whole {
		t.Errorf("the edit still holds rows back after the click:\n%s", r.drawn())
	}
}

func TestGitPatch(t *testing.T) {
	for name, tc := range map[string]struct{ diff, want string }{
		"a provider's diff gets git's line": {
			"--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n",
			"diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1 @@\n-a\n+b\n"},
		"a new file is named by its new side": {
			"--- /dev/null\n+++ b/x.go\n@@ -0,0 +1 @@\n+a\n",
			"diff --git a/x.go b/x.go\n--- /dev/null\n+++ b/x.go\n@@ -0,0 +1 @@\n+a\n"},
		"a removed line reading --- stays in its hunk": {
			"--- a/x.md\n+++ b/x.md\n@@ -1,2 +1,2 @@\n--- a\n+++ b\n",
			"diff --git a/x.md b/x.md\n--- a/x.md\n+++ b/x.md\n@@ -1,2 +1,2 @@\n--- a\n+++ b\n"},
		"each file of a patch gets its own": {
			"--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n--- a/y\n+++ /dev/null\n@@ -1 +0,0 @@\n-c\n",
			"diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\ndiff --git a/y b/y\n--- a/y\n+++ /dev/null\n@@ -1 +0,0 @@\n-c\n"},
		"git's own is left alone": {
			"diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n",
			"diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n"},
	} {
		if got := gitPatch(tc.diff); got != tc.want {
			t.Errorf("%s: gitPatch = %q, want %q", name, got, tc.want)
		}
	}
}

// git's lines over a file's first hunk are not the file's: a diff that falls
// back to its lines as they came leaves them out.
func TestSplitDiffLeavesGitsHeaderOut(t *testing.T) {
	_, added, deleted, body := splitDiff("diff --git a/x b/x\nindex 1..2 100644\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n")
	if added != 1 || deleted != 1 || len(body) != 3 || body[0] != "@@ -1 +1 @@" {
		t.Errorf("splitDiff = +%d -%d %q, want +1 -1 and the hunk alone", added, deleted, body)
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

// The files a call changed with no diff of its own, a shell command's edits,
// stand after it as edit rows, one a file, as an edit tool's do.
func TestAShellCommandsFilesStandAsEdits(t *testing.T) {
	items := []backend.ChatItem{
		{ID: "u", Kind: api.ItemUser, Turn: 1, Text: "go"},
		{ID: "b", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", Title: "sed -i s/a/b/ x.go y.go", State: api.ToolDone,
			Files: []api.ChatFileChange{{Path: "x.go", Diff: "+b"}, {Path: "y.go", Diff: "-a"}}}},
		{ID: "a", Kind: api.ItemAgent, Turn: 1, Text: "done"},
	}
	var got []string
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		switch r.kind {
		case chatRowWork:
			got = append(got, "work")
		case chatRowEdit:
			got = append(got, r.item.ID+":"+r.item.Tool.Diff)
		case chatRowItem:
			got = append(got, r.item.ID)
		}
	}
	want := []string{"u", "work", "b#x.go:+b", "b#y.go:-a", "a"}
	if !slices.Equal(got, want) {
		t.Errorf("rows %v, want %v", got, want)
	}
}
