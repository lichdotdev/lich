package main

import (
	"image"
	"slices"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
)

// findRig lays the bar and the code view out through an input.Router, in the
// preview's order: the bar's chords, the bar, then the view.
type findRig struct {
	router input.Router
	view   *godemirror.View
	bar    *findBar
	th     *ui.Theme
	root   ui.Root
}

func newFindRig(t *testing.T, content string) *findRig {
	t.Helper()
	view, err := newCodeView(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	view.SetContent(godemirror.Content{Filename: "a.go", Text: content})
	r := &findRig{view: view, bar: newFindBar(),
		th: &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}}
	r.frame(func(gtx C) { gtx.Execute(key.FocusCmd{Tag: view}) })
	r.frame(nil)
	return r
}

func (r *findRig) frame(before func(C)) {
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Source: r.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(600, 400))}
	if before != nil {
		before(gtx)
	}
	r.bar.keys(gtx, r.view)
	if r.bar.open {
		r.bar.layout(gtx, r.th, &r.root, r.view)
		r.bar.layoutList(gtx, r.th, r.view)
	}
	if _, err := r.view.Layout(gtx); err != nil {
		panic(err)
	}
	r.router.Frame(ops)
}

func (r *findRig) press(name key.Name, mods key.Modifiers) {
	r.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press})
	r.frame(nil)
	r.frame(nil)
}

func (r *findRig) status(t *testing.T) string {
	t.Helper()
	s, _ := r.bar.status(r.view)
	return s
}

func TestCtrlFOnTheCodeFindsTheSelectionAndEnterWalksTheMatches(t *testing.T) {
	r := newFindRig(t, "shown\nx shown y\nshown")
	r.view.Select(godemirror.Pos{Line: 1, Col: 2}, godemirror.Pos{Line: 1, Col: 7})
	r.press(keyFind, key.ModShortcut)
	if !r.bar.open || r.bar.query.Text() != "shown" {
		t.Fatalf("after Ctrl+F: open %v, query %q; want open on the selection", r.bar.open, r.bar.query.Text())
	}
	if !r.router.Source().Focused(&r.bar.query) {
		t.Error("the field does not hold the focus")
	}
	if got := r.status(t); got != "2 of 3" {
		t.Fatalf("status = %q, want the selection's own match, 2 of 3", got)
	}
	r.press(key.NameReturn, 0)
	if got := r.status(t); got != "3 of 3" {
		t.Errorf("after Enter: %q, want 3 of 3", got)
	}
	r.press(key.NameReturn, 0)
	if got := r.status(t); got != "1 of 3" {
		t.Errorf("Enter past the last: %q, want it to wrap to 1 of 3", got)
	}
	r.press(key.NameReturn, key.ModShift)
	if got := r.status(t); got != "3 of 3" {
		t.Errorf("Shift+Enter from the first: %q, want it to wrap to 3 of 3", got)
	}
}

func TestEscapeClosesTheBarClearsTheMatchesAndHandsTheFocusBack(t *testing.T) {
	r := newFindRig(t, "shown\nshown")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("shown")
	r.frame(nil)
	if n := len(r.view.Matches()); n != 2 {
		t.Fatalf("matches = %d, want 2", n)
	}
	r.press(key.NameEscape, 0)
	if r.bar.open {
		t.Error("the bar is still open")
	}
	if n := len(r.view.Matches()); n != 0 {
		t.Errorf("matches after Escape = %d, want none lit", n)
	}
	if !r.router.Source().Focused(r.view) {
		t.Error("the code does not have the focus back")
	}
}

func TestTypingOnKeepsTheMatchUnderWayInsteadOfSkippingPastIt(t *testing.T) {
	r := newFindRig(t, "abc abc")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("a")
	r.frame(nil)
	r.press(key.NameReturn, 0)
	if got := r.status(t); got != "2 of 2" {
		t.Fatalf("after Enter: %q, want 2 of 2", got)
	}
	r.bar.query.SetText("ab")
	r.frame(nil)
	if got := r.status(t); got != "2 of 2" {
		t.Errorf("after typing on: %q, want the same match, 2 of 2", got)
	}
}

func TestTheTogglesNarrowTheSearch(t *testing.T) {
	r := newFindRig(t, "Shown shown shownly")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("shown")
	r.frame(nil)
	if n := len(r.view.Matches()); n != 3 {
		t.Fatalf("plain: %d matches, want 3", n)
	}
	r.bar.matchCase.Value = true
	r.frame(nil)
	if n := len(r.view.Matches()); n != 2 {
		t.Errorf("match case: %d matches, want 2", n)
	}
	r.bar.wholeWord.Value = true
	r.frame(nil)
	if n := len(r.view.Matches()); n != 1 {
		t.Errorf("match case and whole word: %d matches, want 1", n)
	}
}

func TestTheBarSaysWhyNothingMatches(t *testing.T) {
	r := newFindRig(t, "shown")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("shwon")
	r.frame(nil)
	if got, failed := r.bar.status(r.view); got != "No results" || !failed {
		t.Errorf("no match: %q (failed %v), want No results", got, failed)
	}
	r.bar.regexp.Value = true
	r.bar.query.SetText("show([")
	r.frame(nil)
	if got, failed := r.bar.status(r.view); got != "Invalid regex" || !failed {
		t.Errorf("bad regexp: %q (failed %v), want Invalid regex", got, failed)
	}
}

func TestANewViewGetsTheSearchAgain(t *testing.T) {
	r := newFindRig(t, "shown shown")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("shown")
	r.frame(nil)
	view, err := newCodeView(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	view.SetContent(godemirror.Content{Filename: "a.go", Text: "shown shown"})
	r.view = view
	r.frame(nil)
	if n := len(r.view.Matches()); n != 2 {
		t.Errorf("the new view lights %d matches, want 2", n)
	}
}

func TestFindAllListsEachLineOnceWithItsMatches(t *testing.T) {
	r := newFindRig(t, "shown shown\nnone\n\tshown")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("shown")
	r.frame(nil)
	lines := linesOf(r.view.Matches())
	if len(lines) != 2 || lines[0].line != 0 || len(lines[0].matches) != 2 || lines[1].line != 2 {
		t.Fatalf("lines = %+v, want line 0 with two matches and line 2 with one", lines)
	}
}

func TestFindAllRowLandsTheCodeOnItsLine(t *testing.T) {
	r := newFindRig(t, "shown\nnone\nx shown")
	r.press(keyFind, key.ModShortcut)
	r.bar.query.SetText("shown")
	r.bar.listing.Value = true
	r.frame(nil)
	r.frame(func(gtx C) { gtx.Execute(key.FocusCmd{Tag: &r.bar.list.clicks[1]}) })
	// A Clickable clicks on Enter's release, as a button does.
	r.router.Queue(key.Event{Name: key.NameReturn, State: key.Press}, key.Event{Name: key.NameReturn, State: key.Release})
	r.frame(nil)
	r.frame(nil)
	if got := r.status(t); got != "2 of 2" {
		t.Errorf("after opening the second row: %q, want its match current, 2 of 2", got)
	}
}

func TestMatchSegmentsCutAtRuneColumnsAndDropTheIndent(t *testing.T) {
	text := "\t\té shown, shown"
	matches := []godemirror.Match{
		{From: godemirror.Pos{Col: 4}, To: godemirror.Pos{Col: 9}},
		{From: godemirror.Pos{Col: 11}, To: godemirror.Pos{Col: 16}},
	}
	want := []segment{{text: "é "}, {text: "shown", hit: true}, {text: ", "}, {text: "shown", hit: true}}
	if got := matchSegments(text, matches); !slices.Equal(got, want) {
		t.Errorf("segments = %+v, want %+v", got, want)
	}
}

// The terminal takes the keyboard every frame the Code tab says it does not
// hold it, so the code and the find bar have to count.
func TestCodeTabHoldsTheKeysWhileTheCodeOrTheFindBarHasThem(t *testing.T) {
	r := newFindRig(t, "shown\nnone\nshown")
	c := &codeTab{preview: &preview{view: r.view, find: r.bar}}
	holds := func() bool { return c.focused(layout.Context{Source: r.router.Source()}) }
	if !holds() {
		t.Error("the focused code leaves the keys to the terminal")
	}
	r.press(keyFind, key.ModShortcut)
	if !holds() {
		t.Error("the find bar's field leaves the keys to the terminal")
	}
	r.bar.query.SetText("shown")
	r.bar.listing.Value = true
	r.frame(nil)
	r.frame(func(gtx C) { gtx.Execute(key.FocusCmd{Tag: &r.bar.list.clicks[0]}) })
	if !holds() {
		t.Error("a Find all row leaves the keys to the terminal")
	}
}
