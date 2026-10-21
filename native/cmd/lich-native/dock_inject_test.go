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
	if (&terminals{}).submitter("")("x") {
		t.Error("a session-less submit reported taken")
	}
	shell := &terminals{byID: map[string]*termView{"s": {id: "s", kind: "shell"}}}
	if shell.submitter("s")("x") {
		t.Error("a submit to a shell reported taken")
	}
}

// On a pull request the menu also offers a comment for GitHub, which opens
// the box that files into the pending review.
func TestReviewCommentEntryOpensTheReviewBox(t *testing.T) {
	opened := false
	if e := reviewCommentEntry("", func() { opened = true }); !e.Disabled || e.Label != "Comment on the pull request…" {
		t.Errorf("without a selection: %+v", e)
	}
	e := reviewCommentEntry("3-4", func() { opened = true })
	if e.Disabled || e.Label != "Comment on the pull request 3-4…" {
		t.Errorf("with a selection: %+v", e)
	}
	e.OnClick()
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
