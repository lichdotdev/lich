package main

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/gioterm"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

func TestCodeRefMatches(t *testing.T) {
	files := []string{
		"billing/src/main/java/OrderService.java",
		"orders/src/main/java/com/acme/OrderService.java",
		"src/a.go",
		"web/index.ts",
		"docs/index.ts",
	}
	abs, err := filepath.Abs("checkout")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		ref  gioterm.CodeRef
		want []string
	}{
		{"exact path", gioterm.CodeRef{Path: "src/a.go"}, []string{"src/a.go"}},
		{"dotted path", gioterm.CodeRef{Path: "./src/a.go"}, []string{"src/a.go"}},
		{"absolute inside", gioterm.CodeRef{Path: filepath.Join(abs, "src", "a.go")}, []string{"src/a.go"}},
		{"absolute outside", gioterm.CodeRef{Path: filepath.Join(filepath.Dir(abs), "x", "a.go")}, nil},
		{"parent", gioterm.CodeRef{Path: "../src/a.go"}, nil},
		{"bare name", gioterm.CodeRef{Path: "a.go"}, []string{"src/a.go"}},
		{"no such file", gioterm.CodeRef{Path: "b.go"}, nil},
		{"tie", gioterm.CodeRef{Path: "index.ts"}, []string{"web/index.ts", "docs/index.ts"}},
		{"package settles a tie", gioterm.CodeRef{Path: "OrderService.java", Package: "com/acme"}, []string{"orders/src/main/java/com/acme/OrderService.java"}},
		{"package matching nothing keeps the tie", gioterm.CodeRef{Path: "index.ts", Package: "com/acme"}, []string{"web/index.ts", "docs/index.ts"}},
		{"name is not a tail of a longer name", gioterm.CodeRef{Path: "Service.java"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := codeRefMatches(files, abs, c.ref); !slices.Equal(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestCodeLabelNamesTheFileAndItsLines(t *testing.T) {
	cases := []struct {
		target codeTarget
		want   string
	}{
		{codeTarget{rel: "src/a.go"}, "a.go"},
		{codeTarget{rel: "src/a.go", line: 7}, "a.go:7"},
		{codeTarget{rel: "src/a.go", line: 7, end: 9}, "a.go:7-9"},
	}
	for _, c := range cases {
		if got := codeLabel(c.target); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.target, got, c.want)
		}
	}
}

// An agent that edits a file and points at it again has to show the edit.
func TestOpeningTheSameFileAgainReadsItAgain(t *testing.T) {
	view, err := codeview.New(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	p := newPreview(nil, view, func() {})
	p.openAt("a.go", 7)
	first := p.fileKey("/c")
	p.openAt("a.go", 7)
	if p.fileKey("/c") == first {
		t.Fatal("a second open of a.go reuses the first read")
	}
}

func TestAgentNoteHangsUnderTheLastLineAndGoesWithTheFile(t *testing.T) {
	view, err := codeview.New(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	p := newPreview(nil, view, func() {})
	p.openLines("a.go", 2, 3)
	if blocks := p.blocksWith(-1); blocks != nil {
		t.Fatalf("no note, no composer: %d blocks", len(blocks))
	}
	p.note = agentNote{text: "why"}
	p.show("k", "one\ntwo\nthree\nfour\n")
	if len(p.content.Blocks) != 1 || p.content.Blocks[0].After != 2 {
		t.Fatalf("blocks = %+v, want one under line index 2", p.content.Blocks)
	}
	p.openAt("b.go", 1)
	if p.note.text != "" {
		t.Fatal("the note followed the panel to another file")
	}
}

func TestNoteAndComposerNeverShareALineAsTwoBlocks(t *testing.T) {
	view, err := codeview.New(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	p := newPreview(nil, view, func() {})
	p.openLines("a.go", 5, 7)
	if got := p.blocksWith(2); len(got) != 1 || got[0].After != 2 {
		t.Fatalf("composer alone: %+v", got)
	}
	p.note = agentNote{text: "why"}
	if got := p.blocksWith(6); len(got) != 1 || got[0].After != 6 {
		t.Fatalf("same line: %+v", got)
	}
	got := p.blocksWith(2)
	if len(got) != 2 || got[0].After != 2 || got[1].After != 6 {
		t.Fatalf("different lines: %+v", got)
	}
}

func TestOpenLinesSelectsARangeAndOpenAtForgetsIt(t *testing.T) {
	view, err := codeview.New(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	p := newPreview(nil, view, func() {})
	p.openLines("a.go", 2, 3)
	p.show("k", "one\ntwo\nthree\nfour\n")
	from, to, ok := p.view.SelectedLines()
	if !ok || from != 1 || to != 2 {
		t.Fatalf("selected %d..%d (%v), want 1..2", from, to, ok)
	}
	p.openAt("a.go", 4)
	if p.endLine != 0 {
		t.Fatalf("endLine %d survived a plain open", p.endLine)
	}
}
