package main

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

var (
	icTreeOpen       = icons.Lucide("chevron-down")
	icTreeClosed     = icons.Lucide("chevron-right")
	icTreeFolder     = icons.Lucide("folder")
	icTreeFolderOpen = icons.Lucide("folder-open")
)

// FileTree.tsx geometry: rows indented 0.75rem a level from a 0.5rem base,
// size-3.5 glyphs, gap-1.5, py-0.5 pr-2, the tree's own py-1.
var (
	treeIndent     = ui.Space(3)
	treeIndentBase = ui.Space(2)
	treeGlyph      = ui.Space(3.5)
)

// treeRows is the tree on screen: its list, its rows' clickables, and the
// tree built from the last listing and filter, kept until either moves.
type treeRows struct {
	list   widget.List
	clicks map[string]*widget.Clickable

	builtFrom *treeData
	builtFor  string
	tree      []*treeNode
	stats     map[string]lineDelta
}

func (t *treeRows) click(path string) *widget.Clickable {
	if t.clicks == nil {
		t.clicks = map[string]*widget.Clickable{}
	}
	c, ok := t.clicks[path]
	if !ok {
		c = new(widget.Clickable)
		t.clicks[path] = c
	}
	return c
}

// build is the tree for data narrowed by query, rebuilt only when either
// changed: a big checkout is thousands of paths to nest every frame.
func (t *treeRows) build(data *treeData, query string) {
	if data == t.builtFrom && query == t.builtFor {
		return
	}
	t.builtFrom, t.builtFor = data, query
	t.tree = buildTree(filterPaths(data.listing.Files, query))
	t.stats = withDirStats(t.tree, data.stats)
}

// treeBody is TreeBody: the states the tree reads as, the rows, and the
// footnote a listing that left something out owes the reader.
func (c *codeTab) treeBody(gtx C, th *ui.Theme) D {
	data, loading, err := c.tree.get()
	switch {
	case err != nil:
		return notice(gtx, th, codeReadFailed)
	case loading || data == nil:
		return notice(gtx, th, codeLoading)
	}
	query := c.query.Text()
	filtering := strings.TrimSpace(query) != ""
	c.rows.build(data, query)
	rows := visibleRows(c.rows.tree, func(path string) bool { return filtering || c.toggled[path] })
	c.treeClicks(gtx, rows)
	note := treeFootnote(data.listing.Cut, data.listing.Hidden)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			if len(rows) == 0 {
				empty := codeEmpty
				if filtering {
					empty = codeNoMatch
				}
				return notice(gtx, th, empty)
			}
			return c.treeList(gtx, th, rows)
		}),
		layout.Rigid(func(gtx C) D {
			if note == "" {
				return D{}
			}
			return footnote(gtx, th, note)
		}),
	)
}

// treeClicks folds a folder or opens a file, before the rows are drawn.
func (c *codeTab) treeClicks(gtx C, rows []treeRow) {
	for _, r := range rows {
		if !c.rows.click(r.node.path).Clicked(gtx) {
			continue
		}
		if r.node.dir {
			c.toggled[r.node.path] = !c.toggled[r.node.path]
			continue
		}
		c.open(r.node.path, 0)
	}
}

func (c *codeTab) treeList(gtx C, th *ui.Theme, rows []treeRow) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.ScrollArea(&c.rows.list).Layout(gtx, len(rows), func(gtx C, i int) D {
			return c.treeRow(gtx, th, rows[i])
		})
	})
}

// treeRow is one TreeRow: a file muted until hovered or current, a folder in
// font-medium foreground, each with its +/- at the right.
func (c *codeTab) treeRow(gtx C, th *ui.Theme, r treeRow) D {
	click := c.rows.click(r.node.path)
	lit := click.Hovered() || (!r.node.dir && r.node.path == c.selected)
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if lit {
					return ui.Fill(gtx, th.Accent, 0)
				}
				return D{Size: gtx.Constraints.Min}
			},
			func(gtx C) D {
				on := th
				if lit {
					on = th.On(th.Accent)
				}
				indent := treeIndentBase + treeIndent*unit.Dp(r.depth)
				return layout.Inset{Left: indent, Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
					return c.treeRowContent(gtx, on, r, lit)
				})
			},
		)
	})
}

func (c *codeTab) treeRowContent(gtx C, th *ui.Theme, r treeRow, lit bool) D {
	name := th.Text(ui.TextXS, r.node.name).Mono()
	var lead [2]layout.Widget
	if r.node.dir {
		chevron, folder := icTreeClosed, icTreeFolder
		if r.open {
			chevron, folder = icTreeOpen, icTreeFolderOpen
		}
		lead[0] = func(gtx C) D { return chevron.Layout(gtx, treeGlyph, th.MutedForeground) }
		lead[1] = func(gtx C) D { return folder.Layout(gtx, treeGlyph, th.MutedForeground) }
		name = name.Weight(font.Medium)
	} else {
		lead[0] = layout.Spacer{Width: treeGlyph, Height: treeGlyph}.Layout
		lead[1] = func(gtx C) D { return fileIcon(gtx, th, r.node.path) }
		if !lit {
			name = name.Muted()
		}
	}
	stat, changed := c.rows.stats[r.node.path]
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(lead[0]),
		ui.Gap(1.5),
		layout.Rigid(lead[1]),
		ui.Gap(1.5),
		layout.Flexed(1, func(gtx C) D {
			// ponytail: a name past the dock's width is cut, where the web's
			// tree scrolls sideways; a horizontal scroll when deep trees need it.
			name.MaxLines = 1
			return name.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if !changed {
				return D{}
			}
			return layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D { return diffStat(gtx, th, stat.added, stat.deleted) })
		}),
	)
}

// fileIcon is FileIcon.tsx's fallback, the muted file glyph at 70% opacity.
// ponytail: every file draws it; the web's per-language devicons are
// multi-colored brand marks the kit's one-color icons cannot draw yet.
func fileIcon(gtx C, th *ui.Theme, _ string) D {
	return icFileGlyph.Layout(gtx, treeGlyph, th.Over(ui.Alpha(th.MutedForeground, fileIconOpacity)))
}

const fileIconOpacity = 0.7

var icFileGlyph = icons.Lucide("file")
