package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
)

const blameHash = "1234567890abcdef1234567890abcdef12345678"

var blameNow = time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

func someBlame() api.GitBlame {
	return api.GitBlame{
		Lines:   []string{blameHash, "", blameHash},
		Commits: map[string]api.GitBlameCommit{blameHash: {Author: "omartelo", Time: blameNow.Add(-5 * 24 * time.Hour).Unix(), Summary: "feat: YouTrack board", Message: "feat: YouTrack board\n\nCards and issues."}},
	}
}

func TestBlameNamesTheCommitOrTheUserForEachLine(t *testing.T) {
	b := someBlame()
	kept := []int{0, 1, -1, 2, 7}
	cases := []struct {
		line       int
		text, hash string
	}{
		{0, "omartelo, 5d ago · feat: YouTrack board", blameHash},
		{1, "You · Uncommitted changes", ""},
		{2, "You · Uncommitted changes", ""},
		{3, "omartelo, 5d ago · feat: YouTrack board", blameHash},
		{4, "You · Uncommitted changes", ""},
		{9, "", ""},
	}
	for _, c := range cases {
		if text, hash := blameLine(b, kept, c.line, blameNow); text != c.text || hash != c.hash {
			t.Errorf("line %d: %q %q, want %q %q", c.line, text, hash, c.text, c.hash)
		}
	}
}

func TestBlameAgeRunsToMonthsAndYears(t *testing.T) {
	ago := func(d time.Duration) string { return blameAge(blameNow.Add(-d).Unix(), blameNow) }
	day := 24 * time.Hour
	if got := ago(3 * day); got != "3d ago" {
		t.Errorf("3 days: %q", got)
	}
	if got := ago(70 * day); got != "2mo ago" {
		t.Errorf("70 days: %q", got)
	}
	if got := ago(800 * day); got != "2y ago" {
		t.Errorf("800 days: %q", got)
	}
}

func TestALongBlameIsCutToTheRoomPastTheLine(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
	f := &fileEditor{th: th}
	f.blame.text, f.blame.cells = "omartelo, 5d ago · "+strings.Repeat("feat: YouTrack board, issues, card screen ", 6), 30
	g := godemirror.Geometry{GutterWidth: 40, TextX: 44, CellWidth: 8}
	draw := func(code int) D {
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(code, 20)}}
		return f.blameLabel(gtx, g)
	}
	// The editor draws the blame a cell past the line's 30 cells, at
	// 44+31*8 = 292 from its left edge; with the gutter's 40 the code area
	// of 600 ends at 640.
	if room := blameRoom(600, 30, g); room != 348 {
		t.Fatalf("room %d, want 640-292", room)
	}
	if d := draw(600); d.Size.X == 0 || d.Size.X > 348 {
		t.Fatalf("drawn %dpx wide, want cut to the 348px past the line", d.Size.X)
	}
	if d := draw(300); d.Size.X != 0 {
		t.Fatalf("drawn %dpx wide with no room past the line, want nothing", d.Size.X)
	}
}

func TestTheGutterRuleIsTheGuttersLastColumn(t *testing.T) {
	ed, err := codeview.NewTextEditor(fonts.SystemMono, 13)
	if err != nil {
		t.Fatal(err)
	}
	if err := ed.SetState(godemirror.EditorStateConfig{Doc: strings.Repeat("x\n", 120)}); err != nil {
		t.Fatal(err)
	}
	gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(400, 300))}
	if _, err := ed.Layout(gtx); err != nil {
		t.Fatal(err)
	}
	g := ed.Geometry()
	if g.GutterWidth == 0 {
		t.Fatal("the editor laid out no gutter")
	}
	if got, want := gutterRule(g, 300), image.Rect(g.GutterWidth-1, 0, g.GutterWidth, 300); got != want {
		t.Fatalf("rule %v, want %v: 1px on the gutter's right edge, the code area's height", got, want)
	}
}

func TestMovingTheCursorReadsTheBlameItHolds(t *testing.T) {
	srv, d := diskBackend(t)
	f := loadedEditor(t, srv, d, "one\ntwo\nthree\n")
	f.th = &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
	f.blame.read.data, f.blame.read.has, f.blame.read.settled = someBlame(), true, true
	f.redecorate(f.th, blameNow)
	if !strings.HasPrefix(f.blame.text, "omartelo") {
		t.Fatalf("line 1 reads %q, want its commit", f.blame.text)
	}
	at := f.ed.State().Line(2).From
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(at, at)})
	f.redecorate(f.th, blameNow)
	if f.blame.text != "You · Uncommitted changes" {
		t.Fatalf("line 2 reads %q, want the uncommitted line", f.blame.text)
	}
	at = f.ed.State().Line(3).From
	_ = f.ed.Dispatch(godemirror.TransactionSpec{Selection: godemirror.SingleSelection(at, at)})
	typeAt(t, f, at, "edited ")
	f.redecorate(f.th, blameNow)
	if f.blame.text != "You · Uncommitted changes" {
		t.Fatalf("an edited committed line reads %q, want the user's", f.blame.text)
	}
	if got := len(f.ed.State().Decorations().Widgets); got != 1 {
		t.Fatalf("%d blame widgets, want the cursor line's alone", got)
	}
}
