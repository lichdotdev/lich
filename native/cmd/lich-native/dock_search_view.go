package main

import (
	"context"
	"fmt"
	"gioui.org/font"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/filetree"
)

// hitList is the search list on screen: its rows and the hits' clickables,
// which take focus for the arrow keys.
type hitList struct {
	list   widget.List
	clicks []ui.Clickable
	hits   int // hits drawn last frame
	rowOf  []int
}

// focus moves focus to hit i of last frame's list, scrolling it in.
func (h *hitList) focus(gtx C, i int) {
	if i < 0 || i >= h.hits {
		return
	}
	gtx.Execute(key.FocusCmd{Tag: &h.clicks[i]})
	row := h.rowOf[i]
	if p := h.list.Position; row < p.First || row >= p.First+p.Count-1 {
		h.list.ScrollTo(row)
	}
}

// nameMatchCap is how many files whose names match the list shows, few enough
// that the lines of the second section stay in view; its header counts them
// all.
const nameMatchCap = 10

type resultKind int

const (
	resultSection resultKind = iota // "Files" or "In content" and a count
	resultNote                      // a line saying the section is not a list yet
	resultFile                      // a file whose name matches
	resultGroup                     // a file's header over its hits
	resultHit                       // a line holding the query
)

// searchRow is one row of the list. Files and hits are the rows that take
// focus: hit is their place among them, the files first.
type searchRow struct {
	kind  resultKind
	group *searchGroup
	path  string
	hit   int
	text  string
	count string
}

// nameMatches is the listing's paths whose names match query, kept for as
// long as neither changes.
func (c *codeTab) nameMatches(data *treeData, query string) []string {
	if data == nil {
		return nil
	}
	if data != c.namesFrom || query != c.namesFor {
		c.names, c.namesFrom, c.namesFor = filterPaths(data.listing.Files, query), data, query
	}
	return c.names
}

// searchBody is SearchResults: the files whose names match the field, at once,
// and the lines holding it once typing settles, grouped per file.
func (c *codeTab) searchBody(gtx C, th *ui.Theme) D {
	query := strings.TrimSpace(c.query.Text())
	settled, next := c.settle.at(query, gtx.Now)
	if !next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	c.fetchSearch(settled)
	tree, _, _ := c.tree.get()
	files := c.nameMatches(tree, query)
	data, loading, err := c.search.get()
	groups := groupMatches(data.Matches)
	if len(files) == 0 && len(groups) == 0 && err == nil && !loading && settled == query {
		return th.Notice(i18n.T("dock.searchResults.noMatch", i18n.Args{"query": query})).Layout(gtx)
	}
	rows := c.searchRows(files, groups, data, loading, err)
	note := strings.TrimSpace(searchFootnote(data) + " " + c.listingNote())
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return c.resultsView(gtx, th, rows, files[:min(len(files), nameMatchCap)], groups, settled)
		}),
		layout.Rigid(func(gtx C) D {
			if note == "" {
				return D{}
			}
			return th.Footnote(note).Layout(gtx)
		}),
	)
}

// fetchSearch re-runs on whatever re-reads the tree, so a file the agent just
// wrote is searched without a keystroke. Only another checkout drops the
// list; a new query keeps the last one until its own lands.
func (c *codeTab) fetchSearch(query string) {
	key, path := "", c.checkout
	if query != "" && c.treeKey != "" {
		key = c.treeKey + " " + query
	}
	c.search.want(key, path, func(ctx context.Context) (searchResult, error) {
		return c.client.Search(ctx, path, query)
	})
}

// listingNote is the tree's footnote, which the search inherits: it reads the
// same listing.
func (c *codeTab) listingNote() string {
	data, _, _ := c.tree.get()
	if data == nil {
		return ""
	}
	return treeFootnote(data.listing.Cut, data.listing.Hidden)
}

func (c *codeTab) resultsView(gtx C, th *ui.Theme, rows []searchRow, shown []string, groups []searchGroup, settled string) D {
	c.hitInput(gtx, shown, groups)
	typed := strings.TrimSpace(c.query.Text())
	return th.ScrollArea(&c.hits.list).Layout(gtx, len(rows), func(gtx C, i int) D {
		r := rows[i]
		switch r.kind {
		case resultSection:
			return sectionHeader(gtx, th, r.text, r.count)
		case resultNote:
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx,
				th.Text(ui.TextXS, r.text).Muted().Layout)
		case resultFile:
			return c.fileRow(gtx, th, r.hit, r.path, typed)
		case resultGroup:
			return hitGroupHeader(gtx, th, *r.group)
		}
		return c.hitRow(gtx, th, r.hit, flatHit(groups, r.hit-len(shown)), settled)
	})
}

// searchRows lays the two sections out as rows and sizes the clickables of
// the files and hits.
func (c *codeTab) searchRows(files []string, groups []searchGroup, data searchResult, loading bool, err error) []searchRow {
	var rows []searchRow
	h := &c.hits
	h.rowOf, h.hits = h.rowOf[:0], 0
	focusable := func() int {
		h.rowOf = append(h.rowOf, len(rows))
		h.hits++
		return h.hits - 1
	}
	inContent := i18n.T("dock.searchResults.inContent")
	if len(files) > 0 {
		rows = append(rows, searchRow{kind: resultSection, text: i18n.T("dock.searchResults.files"), count: namesCount(len(files))})
		for _, p := range files[:min(len(files), nameMatchCap)] {
			rows = append(rows, searchRow{kind: resultFile, path: p, hit: focusable()})
		}
	}
	switch {
	case err != nil:
		rows = append(rows, searchRow{kind: resultSection, text: inContent}, searchRow{kind: resultNote, text: backend.ErrorText(err)})
	case loading:
		rows = append(rows, searchRow{kind: resultSection, text: inContent}, searchRow{kind: resultNote, text: i18n.T("dock.searchResults.searching")})
	case len(groups) > 0:
		rows = append(rows, searchRow{kind: resultSection, text: inContent, count: sectionCount(len(data.Matches), data.Cut)})
	}
	for gi := range groups {
		rows = append(rows, searchRow{kind: resultGroup, group: &groups[gi]})
		for range groups[gi].hits {
			rows = append(rows, searchRow{kind: resultHit, group: &groups[gi], hit: focusable()})
		}
	}
	for len(h.clicks) < h.hits {
		h.clicks = append(h.clicks, ui.Clickable{})
	}
	return rows
}

// namesCount is the files section's count: how many are listed of how many
// match.
func namesCount(total int) string {
	if total > nameMatchCap {
		return fmt.Sprintf("%d/%d", nameMatchCap, total)
	}
	return fmt.Sprint(total)
}

// sectionCount is a section's count; a cut one counts what it holds with a plus.
func sectionCount(n int, cut bool) string {
	if cut {
		return fmt.Sprint(n, "+")
	}
	return fmt.Sprint(n)
}

func flatHit(groups []searchGroup, i int) searchMatch {
	for _, g := range groups {
		if i < len(g.hits) {
			return g.hits[i]
		}
		i -= len(g.hits)
	}
	return searchMatch{}
}

// hitInput opens a clicked file or hit and walks focus through them with the
// arrow keys; Enter is the focused row's own click. The files come first.
func (c *codeTab) hitInput(gtx C, files []string, groups []searchGroup) {
	h := &c.hits
	for i := range h.hits {
		if h.clicks[i].Clicked(gtx) {
			if i < len(files) {
				c.open(files[i], 0)
			} else {
				hit := flatHit(groups, i-len(files))
				c.open(hit.Path, hit.Line)
			}
		}
		for {
			ev, ok := gtx.Event(key.Filter{Focus: &h.clicks[i].Clickable, Name: key.NameDownArrow}, key.Filter{Focus: &h.clicks[i].Clickable, Name: key.NameUpArrow})
			if !ok {
				break
			}
			if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
				step := 1
				if e.Name == key.NameUpArrow {
					step = -1
				}
				h.focus(gtx, i+step)
			}
		}
	}
}

// hitGroupHeader is HitGroup's file row: the icon, the directory muted, the
// name, and the count of hits.
func hitGroupHeader(gtx C, th *ui.Theme, g searchGroup) D {
	dir, base := splitPath(g.path)
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.FileIcon(g.path, filetree.Glyph).Layout),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D {
				if dir == "" {
					return D{}
				}
				return th.Text(ui.TextXS, dir+"/").Mono().Muted().Layout(gtx)
			}),
			layout.Flexed(1, th.Text(ui.TextXS, base).Mono().Layout),
			layout.Rigid(th.Text(ui.TextXS, fmt.Sprint(len(g.hits))).Mono().Muted().Layout),
		)
	})
}

// sectionHeader is the label over a section of the list: its name and, at the
// right, how many rows it holds.
func sectionHeader(gtx C, th *ui.Theme, title, count string) D {
	return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
			layout.Flexed(1, th.Text(ui.Text2XS, strings.ToUpper(title)).Leading(text2XSLeading).Muted().Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(ui.Text2XS, count).Mono().Muted().Layout),
		)
	})
}

// fileRow is a file whose name matches: its icon, the directory muted and the
// name with the query marked.
func (c *codeTab) fileRow(gtx C, th *ui.Theme, i int, path, query string) D {
	dir, base := splitPath(path)
	return c.listRow(gtx, th, i, ui.Space(2), func(on *ui.Theme) layout.Widget {
		return func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(on.FileIcon(path, filetree.Glyph).Layout),
				ui.Gap(1.5),
				layout.Rigid(func(gtx C) D {
					if dir == "" {
						return D{}
					}
					return on.Text(ui.TextXS, dir+"/").Mono().Muted().Layout(gtx)
				}),
				layout.Flexed(1, on.MarkedText(highlightSegments(base, query)).Layout),
			)
		}
	})
}

// hitRow is one hit: its line number in a 4ch column, then the line with the
// query marked.
func (c *codeTab) hitRow(gtx C, th *ui.Theme, i int, hit searchMatch, query string) D {
	return c.listRow(gtx, th, i, ui.Space(6), func(on *ui.Theme) layout.Widget {
		return func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(on.Text(ui.TextXS, padLineNumber(hit.Line)).Mono().Muted().Layout),
				ui.Gap(2),
				layout.Flexed(1, on.MarkedText(highlightSegments(hit.Text, query)).Layout),
			)
		}
	})
}

// listRow is a clickable row of the list, bg-accent/50 hovered and bg-accent
// focused, its body indented left from the edge.
func (c *codeTab) listRow(gtx C, th *ui.Theme, i int, left unit.Dp, body func(on *ui.Theme) layout.Widget) D {
	click := &c.hits.clicks[i]
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		bg := th.Surface
		switch {
		case gtx.Focused(click):
			bg = th.Accent
		case click.Hovered():
			bg = th.Over(ui.Alpha(th.Accent, 0.5))
		}
		on := th.On(bg)
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
			func(gtx C) D {
				return layout.Inset{Left: left, Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, body(on))
			},
		)
	})
}
