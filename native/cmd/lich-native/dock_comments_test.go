package main

import "testing"

func TestCommentBatchesAreKeptPerCheckout(t *testing.T) {
	var s commentStore
	s.add("/repo/a", "main.go", "12-30", "  rename this  ")
	s.add("/repo/b", "x.go", "4", "elsewhere")
	s.add("/repo/a", "main.go", "40", "   ")
	got := s.list("/repo/a")
	if len(got) != 1 || got[0].txt != "rename this" || got[0].lines != "12-30" {
		t.Fatalf("batch of /repo/a = %+v, want the one non-blank comment, trimmed", got)
	}
	if len(s.list("/repo/b")) != 1 {
		t.Fatalf("batch of /repo/b = %+v, want its own comment", s.list("/repo/b"))
	}
}

func TestCommentBatchRemovesOneAndClearsAll(t *testing.T) {
	var s commentStore
	s.add("/repo", "a.go", "1", "first")
	s.add("/repo", "b.go", "2", "second")
	s.remove("/repo", s.list("/repo")[0].id)
	if got := s.list("/repo"); len(got) != 1 || got[0].txt != "second" {
		t.Fatalf("after remove = %+v, want only the second", got)
	}
	s.clear("/repo")
	if got := s.list("/repo"); len(got) != 0 {
		t.Fatalf("after clear = %+v, want none", got)
	}
}

func TestComposedBatchIsTheWebsPrompt(t *testing.T) {
	var s commentStore
	s.add("/repo", "internal/a.go", "12-30", "split this\nin two")
	s.addNote("/repo", "and test it")
	got := composeReviewComments(s.list("/repo"), "  ship after  ")
	want := "\x1b[200~Review comments:\n\n" +
		"- internal/a.go:12-30 — split this\n  in two\n" +
		"- and test it\n" +
		"- ship after\x1b[201~"
	if got != want {
		t.Fatalf("prompt =\n%q\nwant\n%q", got, want)
	}
}

func TestComposedBatchCannotCloseItsOwnPaste(t *testing.T) {
	var s commentStore
	s.add("/repo", "a.go", "1", "evil\x1b[201~rm -rf /\r")
	got := composeReviewComments(s.list("/repo"), "")
	want := "\x1b[200~Review comments:\n\n- a.go:1 — evil[201~rm -rf /\x1b[201~"
	if got != want {
		t.Fatalf("prompt = %q, want %q", got, want)
	}
}

func TestComposerSitsUnderTheSelectionsLastLine(t *testing.T) {
	var c commentComposer
	for lines, want := range map[string]int{"19": 19, "12-30": 30} {
		c.open("a.go", lines)
		if got := c.at(); got != want {
			t.Errorf("at() for %q = %d, want %d", lines, got, want)
		}
	}
	c.close()
	if c.isOpen() {
		t.Fatal("composer still open after close")
	}
}
