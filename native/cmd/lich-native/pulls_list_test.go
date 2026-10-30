package main

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const rowMetaHeight = 24

// rowMetaWidth is the index row's columns at 1x, wide enough for all three.
var rowMetaWidth = int(rowChecksWidth + rowVerdictWidth + rowCheckoutWidth)

var rowMetaNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func rowMetaTheme(t *testing.T) *ui.Theme {
	t.Helper()
	faces, err := fonts.Faces()
	if err != nil {
		t.Fatal(err)
	}
	return &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(append(faces, gofont.Collection()...))), Sans: fonts.Sans, Mono: "Go Mono", Surface: ui.Dark.Sidebar}
}

// inkWidth draws w at a row's width and measures across how many columns a
// pixel close to fg landed.
func inkWidth(t *testing.T, w layout.Widget, fg color.NRGBA) int {
	t.Helper()
	win, err := headless.NewWindow(rowMetaWidth, rowMetaHeight)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(rowMetaWidth, rowMetaHeight)}}
	w(gtx)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, rowMetaWidth, rowMetaHeight))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	near := func(a, b uint8) bool { return max(a, b)-min(a, b) < 24 }
	ink := image.Rectangle{Min: image.Pt(rowMetaWidth, rowMetaHeight)}
	for y := range rowMetaHeight {
		for x := range rowMetaWidth {
			if c := img.RGBAAt(x, y); near(c.R, fg.R) && near(c.G, fg.G) && near(c.B, fg.B) {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return ink.Dx()
}

func requested(author string, threads backend.ThreadTally) backend.PRSummary {
	return backend.PRSummary{Author: author, ReviewDecision: "CHANGES_REQUESTED", Threads: threads,
		UpdatedAt: rowMetaNow.Add(-59 * time.Minute).Format(time.RFC3339)}
}

// A long checkout beside the verdict never cuts it: the verdict is the one
// fact on the row worth acting on, and the checkout gives way.
func TestRowSideKeepsTheVerdictWhole(t *testing.T) {
	th := rowMetaTheme(t)
	whole := inkWidth(t, th.Text(ui.TextXS, i18n.T("pulls.pullsStats.changesRequested")).In(th.Destructive).Layout, th.Destructive)
	pr := requested("ana", backend.ThreadTally{Open: 1, Total: 1})
	got := inkWidth(t, func(gtx C) D { return rowSide(gtx, th, pr, "joao-vitorino-dos-santos_snk-rapid-quartz") }, th.Destructive)
	if got < whole {
		t.Errorf("verdict drew %dpx of its %dpx", got, whole)
	}
}

// GitHub keeps CHANGES_REQUESTED after every thread is resolved; the row then
// reads as waiting on the reviewer, as the pull request's heading does.
func TestRowVerdictFollowsTheThreads(t *testing.T) {
	th := rowMetaTheme(t)
	cases := []struct {
		name    string
		threads backend.ThreadTally
		red     bool
	}{
		{"an open thread is the author's move", backend.ThreadTally{Open: 1, Total: 3}, true},
		{"no threads leaves only the verdict", backend.ThreadTally{}, true},
		{"every thread resolved is the reviewer's move", backend.ThreadTally{Open: 0, Total: 3}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pr := requested("ana", c.threads)
			meta := func(gtx C) D { return rowVerdict(gtx, th, pr) }
			red, amber := inkWidth(t, meta, th.Destructive) > 0, inkWidth(t, meta, th.ToneWait) > 0
			if red != c.red || amber == c.red {
				t.Errorf("red %v amber %v, want red %v", red, amber, c.red)
			}
		})
	}
}

func TestWholeOrNothingDropsWhatWouldBeCut(t *testing.T) {
	wide := func(gtx C) D { return D{Size: image.Pt(50, 10)} }
	gtx := layout.Context{Ops: new(op.Ops), Constraints: layout.Constraints{Max: image.Pt(40, 10)}}
	if d := wholeOrNothing(gtx, wide); d.Size.X != 0 {
		t.Errorf("40px for 50px drew %dpx, want nothing", d.Size.X)
	}
	gtx.Constraints.Max.X = 50
	if d := wholeOrNothing(gtx, wide); d.Size.X != 50 {
		t.Errorf("50px for 50px drew %dpx, want all of it", d.Size.X)
	}
}

// Amber on a row is the verdict waiting on its reviewer; a draft beside it is
// the pull request's state, drawn muted so the two never read as one.
func TestRowMetaDraftLeavesAmberToTheVerdict(t *testing.T) {
	th := rowMetaTheme(t)
	pr := backend.PRSummary{Author: "ana", IsDraft: true, UpdatedAt: rowMetaNow.Add(-5 * time.Hour).Format(time.RFC3339)}
	meta := func(gtx C) D { return rowMeta(gtx, th, pr, rowMetaNow) }
	if amber := inkWidth(t, meta, th.ToneWait); amber > 0 {
		t.Errorf("a draft with no verdict drew %dpx of amber", amber)
	}
	if muted := inkWidth(t, meta, th.MutedForeground); muted == 0 {
		t.Error("the draft badge drew nothing")
	}
}

func TestStepAlongHoldsAtTheEnds(t *testing.T) {
	rows := []backend.PRSummary{{Number: 9}, {Number: 7}, {Number: 4}}
	cases := []struct {
		name         string
		n, delta, to int
	}{
		{"down a row", 9, 1, 7},
		{"up a row", 4, -1, 7},
		{"held at the last", 4, 1, 4},
		{"held at the first", 9, -1, 9},
		{"from a row it does not show, the first", 12, 1, 9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stepAlong(rows, c.n, c.delta); got != c.to {
				t.Errorf("stepAlong(%d, %+d) = %d, want %d", c.n, c.delta, got, c.to)
			}
		})
	}
	if got := stepAlong(nil, 7, 1); got != 0 {
		t.Errorf("no rows stepped to %d", got)
	}
}
