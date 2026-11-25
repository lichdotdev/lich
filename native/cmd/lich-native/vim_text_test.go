package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

func TestVimLinesOfKeepsTheLastBreakAsEol(t *testing.T) {
	cases := []struct {
		doc   string
		lines []string
		eol   bool
	}{
		{"", []string{""}, false},
		{"\n", []string{""}, true},
		{"a\nb\n", []string{"a", "b"}, true},
		{"a\nb", []string{"a", "b"}, false},
		{"a\n\n", []string{"a", ""}, true},
	}
	for _, c := range cases {
		lines, eol := vimLinesOf(c.doc)
		if !slices.Equal(lines, c.lines) || eol != c.eol {
			t.Errorf("%q = %q, %v; want %q, %v", c.doc, lines, eol, c.lines, c.eol)
		}
	}
}

func TestVimSpliceAppliesALinesEvent(t *testing.T) {
	cases := []struct {
		doc         string
		first, last int
		lines       []string
		want        string
	}{
		{"a\nb\n", 1, 1, []string{"new"}, "a\nnew\nb\n"},
		{"a\nb\n", 0, 1, []string{"A"}, "A\nb\n"},
		{"a\nb\n", 0, 2, nil, "\n"},
		{"a\nb", 1, 2, []string{"b", "c"}, "a\nb\nc"},
		{"a\n", 1, 1, []string{"z"}, "a\nz\n"},
		{"a\nb\n", 0, -1, []string{"x"}, "x\n"},
	}
	for _, c := range cases {
		if got := vimSplice(c.doc, c.first, c.last, c.lines); got != c.want {
			t.Errorf("%q [%d,%d) %q = %q; want %q", c.doc, c.first, c.last, c.lines, got, c.want)
		}
	}
}

func TestVimChangeIsTheSpanBetweenWhatBothShare(t *testing.T) {
	ch, ok := vimChange("one two\n", "one 2 two\n")
	if !ok || ch.From != 4 || ch.To != 4 || ch.Insert != "2 " {
		t.Fatalf("got %+v, %v", ch, ok)
	}
	if ch, _ := vimChange("é", "è"); ch.From != 0 || ch.To != 2 || ch.Insert != "è" {
		t.Fatalf("a change inside a character split it: %+v", ch)
	}
	if ch, ok := vimChange("one\n", "one\ntwo\n"); !ok || ch.From != 4 || ch.To != 4 || ch.Insert != "two\n" {
		t.Fatalf("text added at the end: %+v, %v", ch, ok)
	}
	if ch, ok := vimChange("one\ntwo\n", "one\n"); !ok || ch.From != 4 || ch.To != 8 || ch.Insert != "" {
		t.Fatalf("text cut at the end: %+v, %v", ch, ok)
	}
	if _, ok := vimChange("same", "same"); ok {
		t.Fatal("no change reported one")
	}
}

func vimState0(t *testing.T, doc string) godemirror.EditorState {
	t.Helper()
	ed, err := codeview.NewTextEditor(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	if err := ed.SetState(godemirror.EditorStateConfig{Doc: doc, AllowMultipleSelections: true}); err != nil {
		t.Fatal(err)
	}
	return ed.State()
}

func TestVimSelectionDrawsEachVisualMode(t *testing.T) {
	st := vimState0(t, "héllo\nworld\nend\n")
	type rng struct{ anchor, head int }
	cases := []struct {
		name string
		s    vimState
		want []rng
	}{
		{"normal", vimState{mode: "n", row: 2, col: 1}, []rng{{8, 8}}},
		{"charwise forward", vimState{mode: "v", row: 2, col: 1, regions: [][4]int{{1, 2, 1, 6}, {2, 1, 2, 2}}}, []rng{{1, 9}}},
		{"charwise backward", vimState{mode: "v", row: 1, col: 1, regions: [][4]int{{1, 2, 1, 6}, {2, 1, 2, 2}}}, []rng{{9, 1}}},
		{"linewise", vimState{mode: "V", row: 2, col: 0, regions: [][4]int{{1, 1, 1, 6}, {2, 1, 2, 5}}}, []rng{{0, 13}}},
		{"blockwise", vimState{mode: "\x16", row: 2, col: 1, regions: [][4]int{{1, 1, 1, 3}, {2, 1, 2, 2}}}, []rng{{0, 3}, {7, 9}}},
	}
	for _, c := range cases {
		var got []rng
		for _, r := range vimSelection(st, c.s).Ranges() {
			got = append(got, rng{r.Anchor(), r.Head()})
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v; want %v", c.name, got, c.want)
		}
	}
}

func TestVimClickColPutsTheCursorOnTheLastCharacterSelected(t *testing.T) {
	st := vimState0(t, "héllo\n")
	if row, col := vimClickCol(st, 0, 3); row != 1 || col != 1 {
		t.Fatalf("forward drag ended on %d,%d; want é at 1,1", row, col)
	}
	if row, col := vimClickCol(st, 3, 0); row != 1 || col != 0 {
		t.Fatalf("backward drag ended on %d,%d; want 1,0", row, col)
	}
}
