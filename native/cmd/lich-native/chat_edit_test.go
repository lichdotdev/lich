package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/unit"

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
	for _, s := range []string{"app", "page.tsx", "+2", "-1"} {
		if !r.shows(s) {
			t.Errorf("the edit line lacks %q:\n%s", s, r.drawn())
		}
	}
	if r.shows("Edit") || r.shows("EDITED") {
		t.Fatalf("the edit shows its tool's name or a group's head:\n%s", r.drawn())
	}
	if name, folder := labelBox(t, r, "page.tsx").Min, labelBox(t, r, "app").Min; folder.X <= name.X || abs(folder.Y-name.Y) > 4 {
		t.Errorf("the folder starts at %v and the name at %v: the name comes first on the line", folder, name)
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

// Edit rows one after another, an edit tool's or a shell command's files,
// read as one group; a row between them ends it.
func TestEditsOneAfterAnotherShareAGroup(t *testing.T) {
	edit := func(id string) backend.ChatItem {
		return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Edit", State: api.ToolDone, Diff: "+a"}}
	}
	items := []backend.ChatItem{
		{ID: "u", Kind: api.ItemUser, Turn: 1, Text: "go"},
		edit("e1"), edit("e2"),
		{ID: "a", Kind: api.ItemAgent, Turn: 1, Text: "and"},
		edit("e3"),
		{ID: "b", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", State: api.ToolDone,
			Files: []api.ChatFileChange{{Path: "x.go", Diff: "+b"}, {Path: "y.go", Diff: "-a"}}}},
	}
	var got []string
	for _, r := range chatRows(items, claudeKind, false, time.Time{}) {
		if r.kind != chatRowEdit {
			continue
		}
		var ids []string
		for _, it := range r.edits {
			ids = append(ids, it.ID)
		}
		got = append(got, fmt.Sprintf("%s above=%v below=%v head=%v", r.item.ID, r.editAbove, r.editBelow, ids))
	}
	want := []string{
		"e1 above=false below=true head=[e1 e2]",
		"e2 above=true below=false head=[]",
		"e3 above=false below=false head=[]",
		"b#x.go above=false below=true head=[b#x.go b#y.go]",
		"b#y.go above=true below=false head=[]",
	}
	if !slices.Equal(got, want) {
		t.Errorf("edit rows\n%v\nwant\n%v", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A group of edits heads its rows with how many files they changed and all
// their lines, and the head opens Review; each row still opens its own diff.
func TestAGroupOfEditsHeadsItsFilesAndOpensReview(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.dock = new(dock)
	r.theme.Code = godemirror.DarkTheme
	edit := func(id, file string, add, del int) backend.ChatItem {
		var b strings.Builder
		fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n", file, file, del, add)
		for i := range del {
			fmt.Fprintf(&b, "-old %d\n", i)
		}
		for i := range add {
			fmt.Fprintf(&b, "+new %d\n", i)
		}
		return backend.ChatItem{ID: id, Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Edit", Title: "/repo/" + file, State: api.ToolDone, Diff: b.String()}}
	}
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "The flake in TestDownloads is back on macOS. Find the cause and fix it."},
		{ID: "r1", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Bash", Title: "go test ./internal/browser/", State: api.ToolDone}},
		{ID: "r2", Kind: api.ItemTool, Turn: 1, Tool: &backend.ChatToolCall{Name: "Read", Title: "internal/browser/downloads.go", State: api.ToolDone}},
		{ID: "a1", Kind: api.ItemAgent, Turn: 1, Text: "The download finishes before the listener is attached, so the first event is lost on a fast disk."},
		edit("e1", "internal/browser/downloads.go", 18, 6),
		edit("e2", "internal/browser/downloads_test.go", 21, 2),
		edit("e3", "docs/ceilings.md", 1, 4),
		{ID: "a2", Kind: api.ItemAgent, Turn: 1, Text: "Attached the listener before navigating. 200 runs, 0 failures (was 3/200)."},
	})
	r.settle(t)
	writeShot(t, r, "edit-group.png")
	for _, s := range []string{"EDITED", chatEditsCount(3), "+40", "-12", "downloads.go", "internal/browser", "+18", "ceilings.md", "docs"} {
		if !r.shows(s) {
			t.Errorf("the group lacks %q:\n%s", s, r.drawn())
		}
	}
	r.clickLabel(t, chatEditsCount(3))
	if u.dock.tab != dockReview {
		t.Errorf("the group's head opened tab %v, want Review", u.dock.tab)
	}
	r.clickLabel(t, "ceilings.md")
	r.settle(t)
	if f := u.chats.pane("s1").fold("edit:e3"); !f.open {
		t.Error("a click on a row of the group left its diff shut")
	}
}

// The answer starts 32dp under the person's message and the next turn 48dp
// under the last, more as the prose grows; a group's rows touch.
func TestTheTurnsRhythmFollowsTheProse(t *testing.T) {
	user := chatRow{kind: chatRowItem, item: backend.ChatItem{Kind: api.ItemUser}}
	answer := chatRow{kind: chatRowItem, item: backend.ChatItem{Kind: api.ItemAgent}}
	work := chatRow{kind: chatRowWork}
	joined := chatRow{kind: chatRowEdit, editAbove: true}
	for _, c := range []struct {
		sp      float32
		r, prev chatRow
		want    unit.Dp
		what    string
	}{
		{1, work, user, 32, "an answer's first row"},
		{1, user, answer, 48, "a turn's start"},
		{1, answer, work, chatRowGap, "a row inside a turn"},
		{1, joined, answer, 0, "an edit joining its group"},
		{2, answer, user, 54, "an answer's first row at twice the prose"},
		{2, user, answer, 84, "a turn's start at twice the prose"},
	} {
		gtx := C{Metric: unit.Metric{PxPerDp: 1, PxPerSp: c.sp}}
		if got := chatRowTop(gtx, c.r, c.prev); got != c.want {
			t.Errorf("%s: %v, want %v", c.what, got, c.want)
		}
	}
}
