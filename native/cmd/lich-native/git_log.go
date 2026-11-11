package main

import (
	"image"
	"slices"
	"strconv"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// gitFilterDebounce waits out typing before the history is read again.
	gitFilterDebounce = 250 * time.Millisecond
	// gitLoadAhead is how many rows from the end the next page is asked for.
	gitLoadAhead = 40
	// gitMaxAuthors bounds the Author menu on a history with many hands.
	gitMaxAuthors = 40
)

var (
	gitFilterHeight = ui.Space(12)
	gitFieldHeight  = ui.Space(8)
	gitSearchWidth  = unit.Dp(200)
	gitPathWidth    = unit.Dp(150)
	gitRefMaxWidth  = unit.Dp(170)
	gitAuthorWidth  = unit.Dp(96)
	gitWhenWidth    = unit.Dp(60)
)

// applyTyped takes what was typed in the filter fields once typing stops.
func (g *gitPanel) applyTyped(gtx C) {
	q, f := strings.TrimSpace(g.query.Text()), strings.TrimSpace(g.file.Text())
	if q != g.typedQuery || f != g.typedFile {
		g.typedQuery, g.typedFile, g.typedAt = q, f, gtx.Now
	}
	if q == g.filter.query && f == g.filter.file {
		return
	}
	if at := g.typedAt.Add(gitFilterDebounce); gtx.Now.Before(at) {
		gtx.Execute(op.InvalidateCmd{At: at})
		return
	}
	g.filter.query, g.filter.file = q, f
}

func (g *gitPanel) logColumn(gtx C, th *ui.Theme, view gitLogView, in gitPanelInput) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return g.filterBar(gtx, th, view) }),
		layout.Flexed(1, func(gtx C) D { return g.logBody(gtx, th, view, in) }),
	)
}

func (g *gitPanel) filterBar(gtx C, th *ui.Theme, view gitLogView) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitFilterHeight)))
	field := func(editor *widget.Editor, placeholder string, width unit.Dp) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			in := th.Input(editor, placeholder)
			in.Height, in.Width, in.TextSize = gitFieldHeight, width, ui.TextXS
			return in.Layout(gtx)
		})
	}
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			field(&g.query, i18n.T("git.panel.searchPlaceholder"), gitSearchWidth),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return g.authorFilter(gtx, th, view) }),
			layout.Rigid(func(gtx C) D { return g.dateFilter(gtx, th) }),
			ui.Gap(1),
			field(&g.file, i18n.T("git.panel.pathPlaceholder"), gitPathWidth),
		)
	})
}

// gitAuthors are the authors of the commits loaded, the only ones the window
// can name without walking the whole history.
func gitAuthors(commits []backend.GitCommit) []string {
	seen := map[string]bool{}
	for _, c := range commits {
		seen[c.Author] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names[:min(len(names), gitMaxAuthors)]
}

func (g *gitPanel) authorFilter(gtx C, th *ui.Theme, view gitLogView) D {
	entries := []ui.MenuEntry{{Kind: ui.MenuRadioItem, Label: i18n.T("git.panel.authorAny"), Checked: g.filter.author == "", OnClick: func() { g.filter.author = "" }}}
	for _, name := range gitAuthors(view.commits) {
		entries = append(entries, ui.MenuEntry{Kind: ui.MenuRadioItem, Label: name, Checked: g.filter.author == name, OnClick: func() { g.filter.author = name }})
	}
	label := i18n.T("git.panel.author")
	if g.filter.author != "" {
		label = g.filter.author
	}
	b := th.Button(&g.author, gitCleanLine(label))
	b.Variant, b.Size, b.Icon = ui.VariantGhost, ui.SizeSM, icPullDown
	return th.DropdownMenu(g.root, &g.authorMenu, entries).Layout(gtx, b.Layout)
}

// gitDateLabel names a date filter by how many days back it reaches.
func gitDateLabel(days int) string {
	switch days {
	case 1:
		return i18n.T("git.panel.dateDay")
	case 7:
		return i18n.T("git.panel.dateWeek")
	case 30:
		return i18n.T("git.panel.dateMonth")
	}
	return i18n.T("git.panel.dateAny")
}

func (g *gitPanel) dateFilter(gtx C, th *ui.Theme) D {
	now := gtx.Now
	var entries []ui.MenuEntry
	for _, days := range []int{0, 1, 7, 30} {
		entries = append(entries, ui.MenuEntry{Kind: ui.MenuRadioItem, Label: gitDateLabel(days), Checked: g.sinceDays == days, OnClick: func() {
			g.sinceDays, g.filter.since = days, 0
			if days > 0 {
				g.filter.since = now.AddDate(0, 0, -days).Unix()
			}
		}})
	}
	label := i18n.T("git.panel.date")
	if g.sinceDays != 0 {
		label = gitDateLabel(g.sinceDays)
	}
	b := th.Button(&g.date, label)
	b.Variant, b.Size, b.Icon = ui.VariantGhost, ui.SizeSM, icPullDown
	return th.DropdownMenu(g.root, &g.dateMenu, entries).Layout(gtx, b.Layout)
}

func (g *gitPanel) logBody(gtx C, th *ui.Theme, view gitLogView, in gitPanelInput) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	switch {
	case !view.loaded:
		return gitLogSkeleton(gtx, th)
	case view.err != "":
		return gitNotice(gtx, th, i18n.T("git.panel.loadFailed", i18n.Args{"error": view.err}))
	case len(view.commits) == 0:
		return gitNotice(gtx, th, i18n.T("git.panel.noCommits"))
	}
	list := gitScroll(th, &g.log)
	return list.Layout(gtx, len(view.commits), func(gtx C, i int) D { return g.logRow(gtx, th, view, i, in) })
}

// gitLogSkeleton is the log's rows as bars, in the rows' own measures.
func gitLogSkeleton(gtx C, th *ui.Theme) D {
	row := gtx.Dp(gitRowHeight)
	graph := gitGraphWidth(gtx, 2)
	for y := 0; y+row <= gtx.Constraints.Max.Y; y += row {
		func() {
			defer op.Offset(image.Pt(graph, y)).Push(gtx.Ops).Pop()
			cell := gtx
			cell.Constraints = layout.Exact(image.Pt(max(gtx.Constraints.Max.X-graph, 0), row))
			layout.Center.Layout(cell, th.SkeletonText(ui.TextSM, unit.Dp(0)).Layout)
		}()
	}
	return D{Size: gtx.Constraints.Max}
}

func (g *gitPanel) logRow(gtx C, th *ui.Theme, view gitLogView, i int, in gitPanelInput) D {
	c := view.commits[i]
	r := gitRow(g.rows, c.Hash)
	for r.click.Clicked(gtx) {
		g.selected = c.Hash
		g.detail.closeFile()
	}
	if i >= len(view.commits)-gitLoadAhead {
		g.store.more()
	}
	return r.click.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitRowHeight))
		gtx.Constraints = layout.Exact(size)
		rowFill(gtx, th, c.Hash == g.selected, r.click.Hovered())
		graph := gitGraphWidth(gtx, view.lanes)
		if view.rows != nil {
			drawGitRow(gtx, th, view.rows[i], image.Pt(graph, size.Y))
		}
		left := max(graph+gtx.Dp(ui.Space(1.5)), gtx.Dp(ui.Space(3.5)))
		return layout.Inset{Left: gtx.Metric.PxToDp(left), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return g.logRowText(gtx, th, c, r, in)
		})
	})
}

func (g *gitPanel) logRowText(gtx C, th *ui.Theme, c backend.GitCommit, r *gitRowState, in gitPanelInput) D {
	fixed := func(width unit.Dp, w layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(width), gtx.Constraints.Max.Y))
			return layout.W.Layout(gtx, w)
		})
	}
	when := th.Text(ui.TextXS, gitWhen(c.When, gtx.Now)).Mono().Muted()
	when.Align = text.End
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return th.Text(ui.TextSM, gitCleanLine(c.Subject)).Truncated().Layout(gtx) }),
		layout.Rigid(func(gtx C) D { return g.refCell(gtx, th, c, r, in) }),
		ui.Gap(2),
		fixed(gitAuthorWidth, func(gtx C) D { return th.Text(ui.TextXS, gitCleanLine(c.Author)).Muted().Truncated().Layout(gtx) }),
		fixed(gitWhenWidth, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return when.Layout(gtx)
		}),
	)
}

// refCell is the row's one ref, led by the ring of a session on that branch.
func (g *gitPanel) refCell(gtx C, th *ui.Theme, c backend.GitCommit, r *gitRowState, in gitPanelInput) D {
	label, ok := gitRefLabelOf(c.Refs)
	if !ok {
		return D{}
	}
	text := label.text
	if label.extra > 0 {
		text += " +" + strconv.Itoa(label.extra)
	}
	l := th.Text(ui.TextXS, text).Mono().Muted()
	if label.head {
		l = l.In(th.Foreground)
	}
	sv, onBranch := in.sessions[label.branch]
	return layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if !onBranch {
					return D{}
				}
				return layout.Inset{Right: ui.Space(1)}.Layout(gtx, func(gtx C) D { return g.sessionMark(gtx, th, r, sv) })
			}),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(gitRefMaxWidth))
				return l.Truncated().Layout(gtx)
			}),
		)
	})
}
