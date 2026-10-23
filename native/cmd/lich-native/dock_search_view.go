package main

import (
	"context"
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// hitList is the search list on screen: its rows and the hits' clickables,
// which take focus for the arrow keys.
type hitList struct {
	list   widget.List
	clicks []widget.Clickable
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

// searchRow is one row of the list: the summary, a file's header or a hit.
type searchRow struct {
	group *searchGroup
	hit   int // index into the flat hits, -1 for a header or the summary
}

// searchBody is SearchResults: the settled query's hits, grouped per file.
func (c *codeTab) searchBody(gtx C, th *ui.Theme) D {
	settled, next := c.settle.at(strings.TrimSpace(c.query.Text()), gtx.Now)
	if !next.IsZero() {
		gtx.Execute(op.InvalidateCmd{At: next})
	}
	c.fetchSearch(settled)
	if settled == "" {
		return notice(gtx, th, i18n.T("dock.searchResults.prompt"))
	}
	data, loading, err := c.search.get()
	switch {
	case err != nil:
		return notice(gtx, th, backend.ErrorText(err))
	case loading:
		return notice(gtx, th, i18n.T("dock.searchResults.searching"))
	}
	groups := groupMatches(data.Matches)
	note := strings.TrimSpace(searchFootnote(data) + " " + c.listingNote())
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if len(groups) == 0 {
				return notice(gtx, th, i18n.T("dock.searchResults.noMatch", i18n.Args{"query": settled}))
			}
			return c.hitsView(gtx, th, data, groups)
		}),
		layout.Rigid(func(gtx C) D {
			if note == "" {
				return D{}
			}
			return footnote(gtx, th, note)
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

func (c *codeTab) hitsView(gtx C, th *ui.Theme, data searchResult, groups []searchGroup) D {
	rows := c.searchRows(groups)
	c.hitInput(gtx, groups)
	query := strings.TrimSpace(c.query.Text())
	summary := strings.ToUpper(searchSummary(data, len(groups)))
	return th.ScrollArea(&c.hits.list).Layout(gtx, len(rows), func(gtx C, i int) D {
		r := rows[i]
		switch {
		case r.group == nil:
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx,
				th.Text(text2XS, summary).Leading(text2XSLeading).Muted().Layout)
		case r.hit < 0:
			return hitGroupHeader(gtx, th, *r.group)
		}
		return c.hitRow(gtx, th, r.hit, flatHit(groups, r.hit), query)
	})
}

// searchRows lays the groups out as rows and sizes the hits' clickables.
func (c *codeTab) searchRows(groups []searchGroup) []searchRow {
	rows := []searchRow{{hit: -1}}
	h := &c.hits
	h.rowOf, h.hits = h.rowOf[:0], 0
	for gi := range groups {
		rows = append(rows, searchRow{group: &groups[gi], hit: -1})
		for range groups[gi].hits {
			h.rowOf = append(h.rowOf, len(rows))
			rows = append(rows, searchRow{group: &groups[gi], hit: h.hits})
			h.hits++
		}
	}
	for len(h.clicks) < h.hits {
		h.clicks = append(h.clicks, widget.Clickable{})
	}
	return rows
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

// hitInput opens a clicked hit and walks focus through the hits with the
// arrow keys; Enter is the focused hit's own click.
func (c *codeTab) hitInput(gtx C, groups []searchGroup) {
	h := &c.hits
	for i := range h.hits {
		if h.clicks[i].Clicked(gtx) {
			hit := flatHit(groups, i)
			c.open(hit.Path, hit.Line)
		}
		for {
			ev, ok := gtx.Event(key.Filter{Focus: &h.clicks[i], Name: key.NameDownArrow}, key.Filter{Focus: &h.clicks[i], Name: key.NameUpArrow})
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
			layout.Rigid(func(gtx C) D { return fileIcon(gtx, th, g.path) }),
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

// hitRow is one hit: its line number in a 4ch column, then the line with the
// query marked, bg-accent/50 hovered and bg-accent focused.
func (c *codeTab) hitRow(gtx C, th *ui.Theme, i int, hit searchMatch, query string) D {
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
				return layout.Inset{Left: ui.Space(6), Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
						layout.Rigid(on.Text(ui.TextXS, padLineNumber(hit.Line)).Mono().Muted().Layout),
						ui.Gap(2),
						layout.Flexed(1, func(gtx C) D { return hitText(gtx, on, hit.Text, query) }),
					)
				})
			},
		)
	})
}

// hitText draws the line's segments, a hit on a bg-tone-wait/30 mark.
func hitText(gtx C, th *ui.Theme, text, query string) D {
	segs := highlightSegments(text, query)
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
