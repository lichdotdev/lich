package gioterm

import "testing"

func TestPasteFollowsBracketedPasteMode(t *testing.T) {
	v := newSizedTerminal(t)
	if got := string(v.Paste("a\nb")); got != "a\rb" {
		t.Fatalf("unbracketed paste %q, want newlines as returns", got)
	}
	v.Write([]byte("\x1b[?2004h"))
	if got := string(v.Paste("a\nb")); got != "\x1b[200~a\nb\x1b[201~" {
		t.Fatalf("bracketed paste %q", got)
	}
	// An escape in the text must not end the bracket early.
	if got := string(v.Paste("x\x1b[201~y")); got != "\x1b[200~x [201~y\x1b[201~" {
		t.Fatalf("unsafe paste %q", got)
	}
}

func TestAlternateScrollOnlyOnTheAlternateScreen(t *testing.T) {
	v := newSizedTerminal(t)
	if v.AlternateScroll() {
		t.Fatal("alternate scroll on the primary screen")
	}
	v.Write([]byte("\x1b[?1049h"))
	if !v.AlternateScroll() {
		t.Fatal("no alternate scroll on the alternate screen")
	}
	v.Write([]byte("\x1b[?1007l"))
	if v.AlternateScroll() {
		t.Fatal("alternate scroll after the program turned it off")
	}
}

func TestWriteNothing(t *testing.T) {
	v := newSizedTerminal(t)
	v.Write(nil)
}
