package main

import (
	"errors"
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

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
	want := "Review comments:\n\n" +
		"- internal/a.go:12-30 — split this\n  in two\n" +
		"- and test it\n" +
		"- ship after"
	if got != want {
		t.Fatalf("prompt =\n%q\nwant\n%q", got, want)
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

// The session can refuse a batch after the send left (it stopped, its mod
// said no): the comments stay until the session has taken them.
func TestRefusedBatchKeepsTheComments(t *testing.T) {
	var s commentStore
	s.add("/repo", "main.go", "12", "rename this")
	var b commentBatch
	b.sendBatch(&s, "/repo", func(_ string, done func(error)) bool {
		done(errors.New("the session was stopped"))
		return true
	}, func(ui.Toast) {})
	b.settle(&s)
	if got := s.list("/repo"); len(got) != 1 {
		t.Fatalf("batch after a refused send = %+v, want the comment kept", got)
	}
}

// A taken batch drops what was sent, and only that: a comment written while
// the send was on its way stays for the next one.
func TestTakenBatchDropsOnlyWhatWasSent(t *testing.T) {
	var s commentStore
	s.add("/repo", "main.go", "12", "rename this")
	var b commentBatch
	var done func(error)
	b.sendBatch(&s, "/repo", func(_ string, d func(error)) bool {
		done = d
		return true
	}, func(ui.Toast) {})
	b.settle(&s)
	if got := s.list("/repo"); len(got) != 1 {
		t.Fatalf("batch while the send is on its way = %+v, want the comment kept", got)
	}
	s.add("/repo", "x.go", "4", "written meanwhile")
	done(nil)
	b.settle(&s)
	if got := s.list("/repo"); len(got) != 1 || got[0].txt != "written meanwhile" {
		t.Fatalf("batch after the send was taken = %+v, want only the later comment", got)
	}
}

func TestCommentBatchEditsOneInPlace(t *testing.T) {
	var s commentStore
	s.add("/repo", "a.go", "1", "first")
	s.add("/repo", "b.go", "2", "second")
	s.edit("/repo", s.list("/repo")[0].id, "  first, reworded  ")
	got := s.list("/repo")
	if len(got) != 2 || got[0].txt != "first, reworded" || got[0].path != "a.go" || got[0].lines != "1" || got[1].txt != "second" {
		t.Fatalf("after edit = %+v, want the first reworded and trimmed, in place, the second untouched", got)
	}
}

func TestCommentEditFollowsTheCommentItWasOpenedOn(t *testing.T) {
	var s commentStore
	s.add("/repo", "a.go", "1", "first")
	s.add("/repo", "b.go", "2", "second")
	second := s.list("/repo")[1].id
	s.beginEdit("/repo", second)
	if c, ok := s.edited(); !ok || c.txt != "second" || !s.isEditing() {
		t.Fatalf("edited() = %+v, %v; want the second comment", c, ok)
	}
	s.remove("/repo", second)
	if _, ok := s.edited(); ok {
		t.Fatal("edited() still finds a removed comment")
	}
	s.endEdit()
	if s.isEditing() {
		t.Fatal("still editing after endEdit")
	}
}

func TestCommentBatchMovesWholeAfterWhatItJoins(t *testing.T) {
	var s commentStore
	s.add("/wt", "a.go", "1", "already here")
	s.add("https://pr/1", "b.go", "2", "written before the checkout")
	s.beginEdit("https://pr/1", s.list("https://pr/1")[0].id)
	s.move("https://pr/1", "/wt")
	got := s.list("/wt")
	if len(got) != 2 || got[0].txt != "already here" || got[1].txt != "written before the checkout" {
		t.Fatalf("batch of /wt = %+v, want its own comment then the moved one", got)
	}
	if len(s.list("https://pr/1")) != 0 {
		t.Fatalf("batch left behind = %+v, want none", s.list("https://pr/1"))
	}
	if c, ok := s.edited(); !ok || c.txt != "written before the checkout" {
		t.Fatalf("edited = %+v, %v; want the moved comment still open", c, ok)
	}
}
