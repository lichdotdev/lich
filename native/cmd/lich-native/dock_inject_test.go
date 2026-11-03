package main

import "testing"

func TestInjectReferencesMatchInjectMenu(t *testing.T) {
	if got := fileReference("src/app.go"); got != "@src/app.go " {
		t.Errorf("file = %q", got)
	}
	if got := linesReference("src/app.go", formatLineRef(12, 30)); got != "src/app.go:12-30 " {
		t.Errorf("lines = %q", got)
	}
	if got := formatLineRef(19, 19); got != "19" {
		t.Errorf("one line = %q", got)
	}
}

func TestSafeToWriteDropsEveryControlCharacter(t *testing.T) {
	if got := safeToWrite("@a\nb\x1b[201~c\td\x7f "); got != "@ab[201~cd " {
		t.Errorf("safe = %q", got)
	}
}

func TestInjectEntriesNeedASelectionForLinesAndComment(t *testing.T) {
	var wrote string
	inject := func(s string) bool { wrote = s; return true }
	commented := false
	none := injectEntries("a.go", "", inject, func() { commented = true })
	if none[0].Disabled || !none[1].Disabled || !none[2].Disabled {
		t.Errorf("without a selection: %+v", none)
	}
	some := injectEntries("a.go", "3-4", inject, func() { commented = true })
	if some[1].Label != "Inject lines 3-4" || some[2].Label != "Comment for the session 3-4…" {
		t.Errorf("labels = %q, %q", some[1].Label, some[2].Label)
	}
	some[1].OnClick()
	if wrote != "a.go:3-4 " {
		t.Errorf("wrote %q", wrote)
	}
	some[2].OnClick()
	if !commented {
		t.Error("the comment item opened no composer")
	}
}

func TestDockInjectorRefusesWithoutASession(t *testing.T) {
	if (&terminals{}).injector("")("x") {
		t.Error("a session-less inject reported taken")
	}
	if (&terminals{}).submitter("")("x", func(error) {}) {
		t.Error("a session-less submit reported taken")
	}
	shell := &terminals{byID: map[string]*termView{"s": {id: "s", kind: "shell"}}}
	if shell.submitter("s")("x", func(error) {}) {
		t.Error("a submit to a shell reported taken")
	}
}

// On a pull request the menu also offers a comment for GitHub, which opens
// the box that files into the pending review. The contract changed: with no
// selection, or on the account's own pull request, the entry is left out
// rather than shown disabled.
func TestReviewCommentEntriesOpenTheReviewBox(t *testing.T) {
	opened := false
	open := func() { opened = true }
	if got := reviewCommentEntries("", false, open); len(got) != 0 {
		t.Errorf("without a selection: %+v, want no entry", got)
	}
	if got := reviewCommentEntries("3-4", true, open); len(got) != 0 {
		t.Errorf("on the account's own pull request: %+v, want no entry", got)
	}
	got := reviewCommentEntries("3-4", false, open)
	if len(got) != 1 || got[0].Disabled || got[0].Label != "Comment on the pull request 3-4…" {
		t.Fatalf("with a selection: %+v", got)
	}
	got[0].OnClick()
	if !opened {
		t.Error("the entry opened no box")
	}
}

// A box opened for the pull request files into the review, never the
// session's batch; one opened for the session does the opposite.
func TestComposerFilesWhereItWasOpenedFor(t *testing.T) {
	var reviewed []string
	store := new(commentStore)
	c := commentComposer{toReview: func(path, lines, body string) { reviewed = append(reviewed, path+":"+lines+" "+body) }}
	c.openReview("a.go", "3-4")
	c.editor.SetText("rename this")
	c.commit(store, "/repo")
	if len(reviewed) != 1 || reviewed[0] != "a.go:3-4 rename this" || len(store.list("/repo")) != 0 {
		t.Fatalf("review box filed review %v, batch %v", reviewed, store.list("/repo"))
	}
	if c.isOpen() {
		t.Error("the box stayed open after filing")
	}
	c.open("a.go", "7")
	c.editor.SetText("for the agent")
	c.commit(store, "/repo")
	if len(reviewed) != 1 || len(store.list("/repo")) != 1 {
		t.Fatalf("session box filed review %v, batch %v", reviewed, store.list("/repo"))
	}
}
