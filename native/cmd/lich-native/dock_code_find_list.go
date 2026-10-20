package main

// The find bar's Find all: every line of the file with a match, in the look
// of the Text search's hits, between the bar and the code.

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// findListShare is how much of the panel's height the list may take, so the
// code it moves stays in view under it.
const findListShare = 0.4

type findList struct {
	list   widget.List
	clicks []widget.Clickable
	// current is the line of the current match last frame, -1 for none: the
	// list scrolls to it once it changes.
	current int
}

func newFindList() findList {
	return findList{list: widget.List{List: layout.List{Axis: layout.Vertical}}, current: -1}
}

// findLine is one row of the list: a line and its matches, in order.
type findLine struct {
	line    int
	matches []godemirror.Match
}

func linesOf(matches []godemirror.Match) []findLine {
	var lines []findLine
	for _, m := range matches {
		if n := len(lines); n > 0 && lines[n-1].line == m.From.Line {
			lines[n-1].matches = append(lines[n-1].matches, m)
			continue
		}
		lines = append(lines, findLine{line: m.From.Line, matches: []godemirror.Match{m}})
	}
	return lines
}

// currentLine is the row of the selected match, -1 when the selection is not
// one.
func currentLine(view *godemirror.View, lines []findLine) int {
	from, to, ok := view.Selection()
	if !ok {
		return -1
	}
	for i, l := range lines {
		for _, m := range l.matches {
			if m.From == from && m.To == to {
				return i
			}
		}
	}
	return -1
}

// layout is the list under the bar, the count over the rows; nothing while
// there is no match.
func (l *findList) layout(gtx C, th *ui.Theme, view *godemirror.View) D {
	matches := view.Matches()
	if len(matches) == 0 {
		return D{}
	}
	lines := linesOf(matches)
	for len(l.clicks) < len(lines) {
		l.clicks = append(l.clicks, widget.Clickable{})
	}
	l.input(gtx, view, lines)
	current := currentLine(view, lines)
	if current != l.current {
		l.current = current
		l.reveal(current + 1)
	}
	gtx.Constraints.Min.Y = 0
	gtx.Constraints.Max.Y = int(float32(gtx.Constraints.Max.Y) * findListShare)
	summary := strings.ToUpper(i18n.T("dock.findBar.matches", i18n.Args{"count": len(matches)}))
	return th.ScrollArea(&l.list).Layout(gtx, len(lines)+1, func(gtx C, i int) D {
		if i == 0 {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx,
				th.Text(text2XS, summary).Leading(text2XSLeading).Muted().Layout)
		}
		return l.row(gtx, th, view, i-1, lines[i-1], i-1 == current)
	})
}

// reveal scrolls row in when it is out of the list's view.
func (l *findList) reveal(row int) {
	if row <= 0 {
		return
	}
	if p := l.list.Position; row < p.First || row >= p.First+p.Count-1 {
		l.list.ScrollTo(row)
	}
}

// input lands the code on a clicked line's first match, selected, and walks
// focus through the rows with the arrow keys; Enter is the focused row's own
// click.
func (l *findList) input(gtx C, view *godemirror.View, lines []findLine) {
	for i, line := range lines {
		if l.clicks[i].Clicked(gtx) {
			first := line.matches[0]
			view.Select(first.From, first.To)
			view.ScrollToLine(line.line, godemirror.AlignCenter)
		}
		for {
			ev, ok := gtx.Event(key.Filter{Focus: &l.clicks[i], Name: key.NameDownArrow}, key.Filter{Focus: &l.clicks[i], Name: key.NameUpArrow})
			if !ok {
				break
			}
			e, isKey := ev.(key.Event)
			if !isKey || e.State != key.Press {
				continue
			}
			next := i + 1
			if e.Name == key.NameUpArrow {
				next = i - 1
			}
			if next >= 0 && next < len(lines) {
				gtx.Execute(key.FocusCmd{Tag: &l.clicks[next]})
				l.reveal(next + 1)
			}
		}
	}
}

// row is one line: its number in the 4ch column, then its text with the
// matches marked, bg-accent when focused or the current match's.
func (l *findList) row(gtx C, th *ui.Theme, view *godemirror.View, i int, line findLine, current bool) D {
	click := &l.clicks[i]
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		bg := th.Surface
		switch {
		case gtx.Focused(click) || current:
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, 0.5))
		}
		on := th.On(bg)
		segs := matchSegments(view.LineText(line.line), line.matches)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(on.Text(ui.TextXS, padLineNumber(line.line+1)).Mono().Muted().Layout),
						ui.Gap(2),
						layout.Flexed(1, func(gtx C) D { return markedText(gtx, on, segs) }),
					)
				})
			},
		)
	})
}

// matchSegments cuts text at its matches' columns, which count runes, with
// the line's indentation dropped as the Text search's hits drop it.
func matchSegments(text string, matches []godemirror.Match) []segment {
	runes := []rune(text)
	var out []segment
	at := 0
	for _, m := range matches {
		if m.From.Col > at {
			out = append(out, segment{text: string(runes[at:m.From.Col])})
		}
		out = append(out, segment{text: string(runes[m.From.Col:m.To.Col]), hit: true})
		at = m.To.Col
	}
	if at < len(runes) {
		out = append(out, segment{text: string(runes[at:])})
	}
	if len(out) > 0 && !out[0].hit {
		out[0].text = strings.TrimLeft(out[0].text, " \t")
	}
	return out
}

// markedText draws the segments in one line, a match on a bg-tone-wait/30
// mark.
func markedText(gtx C, th *ui.Theme, segs []segment) D {
	kids := make([]layout.FlexChild, len(segs))
	mark := th.Over(ui.Alpha(th.ToneWait, 0.3))
	for i, s := range segs {
		l := th.Text(ui.TextXS, s.text).Mono()
		l.MaxLines = 1
		if !s.hit {
			kids[i] = layout.Rigid(l.Layout)
			continue
		}
		kids[i] = layout.Rigid(func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, mark, ui.RadiusSM) },
				th.On(mark).Text(ui.TextXS, s.text).Mono().Layout)
		})
	}
	return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, kids...)
}
