package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
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
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("a.go", 7, 0, agentNote{})
	first, nudge := f.fileKey(), f.disk.nudge
	f.open("a.go", 7, 0, agentNote{})
	if f.disk.nudge == nudge {
		t.Fatal("a second open of a.go does not read the disk again")
	}
	f.open("b.go", 1, 0, agentNote{})
	if f.fileKey() == first {
		t.Fatal("another file reuses the first one's read")
	}
}

func TestAgentNoteGoesWithTheFile(t *testing.T) {
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("a.go", 2, 3, agentNote{text: "why"})
	f.open("b.go", 1, 0, agentNote{})
	if f.note.text != "" {
		t.Fatal("the note followed the editor to another file")
	}
}

func TestOpenLinesSelectsARangeAndAPlainOpenForgetsIt(t *testing.T) {
	ed, err := codeview.NewTextEditor(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("a.go", 2, 3, agentNote{})
	f.ed, f.loaded = ed, f.opens
	if err := f.setText("one\ntwo\nthree\nfour\n"); err != nil {
		t.Fatal(err)
	}
	f.landLines()
	if got := f.selectedLines(); got != "2-3" {
		t.Fatalf("selected %q, want 2-3", got)
	}
	f.open("a.go", 4, 0, agentNote{})
	if f.endLine != 0 {
		t.Fatalf("endLine %d survived a plain open", f.endLine)
	}
}

// A file git ignores is not in the listing, and the click still opens it.
func TestClickOpensAnIgnoredFileTheListingLacks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rpc/project.Tree":
			_ = json.NewEncoder(w).Encode(fileListing{Files: []string{"a.go"}})
		case "/rpc/project.ReadFile":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"build/out.log"`) {
				http.Error(w, `{"error":"no such file"}`, http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode("text")
		default:
			t.Errorf("unexpected call %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	c := clientOf(srv)
	checkout := t.TempDir()
	cases := []struct {
		path string
		want []string
	}{
		{"build/out.log", []string{"build/out.log"}},
		{filepath.Join(checkout, "build", "out.log"), []string{"build/out.log"}},
		{"build/missing.log", nil},
		{"../outside.log", nil},
		{filepath.Join(filepath.Dir(checkout), "elsewhere", "out.log"), nil},
		{"a.go", []string{"a.go"}},
	}
	for _, tc := range cases {
		got, err := checkoutMatches(context.Background(), c, checkout, gioterm.CodeRef{Path: tc.path, Line: 3})
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, %v; want %v", tc.path, got, err, tc.want)
		}
	}
}
