package main

import (
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
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

	// The tree scrolls sideways once a row is wider than the column: widest
	// is the widest row the last frame laid out, offsetX how far it scrolled.
	hscroll gesture.Scroll
	hbar    widget.Scrollbar
	vbar    widget.Scrollbar
	widest  int
	measure int
	offsetX int

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
		return notice(gtx, th, i18n.T("dock.filesPanel.readFailed"))
	case loading || data == nil:
		return notice(gtx, th, i18n.T("dock.filesPanel.loading"))
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
				empty := i18n.T("dock.filesPanel.empty")
				if filtering {
					empty = i18n.T("dock.filesPanel.noMatch")
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
	return c.rows.layout(gtx, th, rows, c.selected)
}

// layout is the tree's rows, py-1, scrolling down and, once a row is wider
// than the column, sideways, with its own bar along the bottom. While the
// rows overflow downward they stop short of the vertical bar, as the
// sidebar's Occupy keeps its cards off it.
func (t *treeRows) layout(gtx C, th *ui.Theme, rows []treeRow, selected string) D {
	view := gtx.Constraints.Max
	style := th.ScrollArea(&t.list).ScrollbarStyle
	overflowsY := t.list.Position.Length > view.Y && len(rows) > 0
	content := view
	if overflowsY {
		content.X -= gtx.Dp(style.Width())
	}
	width := max(content.X, t.widest)
	t.offsetX += t.hscroll.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Horizontal,
		pointer.ScrollRange{Min: -t.offsetX, Max: width - content.X - t.offsetX}, pointer.ScrollRange{})
	if d := t.hbar.ScrollDistance(); d != 0 {
		t.offsetX += int(d * float32(width))
	}
	if d := t.vbar.ScrollDistance(); d != 0 {
		t.list.List.ScrollBy(d * float32(len(rows)))
	}
	t.offsetX = min(max(t.offsetX, 0), width-content.X)
	defer clip.Rect{Max: view}.Push(gtx.Ops).Pop()
	t.hscroll.Add(gtx.Ops)
	t.measure = 0
	rowsClip := clip.Rect{Max: content}.Push(gtx.Ops)
	stack := op.Offset(image.Pt(-t.offsetX, 0)).Push(gtx.Ops)
	inner := gtx
	inner.Constraints = layout.Exact(image.Pt(width, view.Y))
	layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(inner, func(gtx C) D {
		return t.list.List.Layout(gtx, len(rows), func(gtx C, i int) D {
			return t.row(gtx, th, rows[i], selected)
		})
	})
	stack.Pop()
	rowsClip.Pop()
	t.widest = t.measure
	bar := func(gtx C, state *widget.Scrollbar, axis layout.Axis, start, end float32) D {
		style.Scrollbar = state
		return style.Layout(gtx, axis, start, end)
	}
	if width > content.X {
		start := float32(t.offsetX) / float32(width)
		under := gtx
		under.Constraints.Max.X = content.X
		layout.S.Layout(under, func(gtx C) D {
			return bar(gtx, &t.hbar, layout.Horizontal, start, start+float32(content.X)/float32(width))
		})
	}
	if overflowsY {
		pos := t.list.Position
		row := pos.Length / len(rows)
		start := float32(pos.First*row+pos.Offset) / float32(pos.Length)
		layout.E.Layout(gtx, func(gtx C) D {
			return bar(gtx, &t.vbar, layout.Vertical, start, start+float32(view.Y)/float32(pos.Length))
		})
	}
	return D{Size: view}
}

// measureRow notes the row's own width, its two ends side by side inside
// inset, which the next frame scrolls the tree by. The ends are measured
// apart: the spring between them would take all the room it is given.
func (t *treeRows) measureRow(gtx C, inset layout.Inset, lead, stat layout.Widget) {
	macro := op.Record(gtx.Ops)
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(treeUnbounded, gtx.Constraints.Max.Y)}
	width := gtx.Dp(inset.Left) + lead(g).Size.X + stat(g).Size.X + gtx.Dp(inset.Right)
	macro.Stop()
	t.measure = max(t.measure, width)
}

// treeUnbounded is the width a row is measured in, wider than any name.
const treeUnbounded = 1 << 20

// row is one TreeRow: a file muted until hovered or current (selected), a
// folder in font-medium foreground, each with its +/- at the right.
func (t *treeRows) row(gtx C, th *ui.Theme, r treeRow, selected string) D {
	click := t.click(r.node.path)
	lit := click.Hovered() || (!r.node.dir && r.node.path == selected)
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
				inset := layout.Inset{Left: indent, Right: ui.Space(2), Top: ui.Space(0.5), Bottom: ui.Space(0.5)}
				lead, stat := t.rowParts(on, r, lit)
				t.measureRow(gtx, inset, lead, stat)
				return inset.Layout(gtx, func(gtx C) D {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(lead),
						layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
						layout.Rigid(stat),
					)
				})
			},
		)
	})
}

// rowParts are a row's two ends: the chevron, glyph and name, and its +/-
// (nothing for an unchanged path), which the row pushes apart.
func (t *treeRows) rowParts(th *ui.Theme, r treeRow, lit bool) (lead, stat layout.Widget) {
	name := th.Text(ui.TextXS, r.node.name).Mono()
	name.MaxLines = 1
	var glyphs [2]layout.Widget
	if r.node.dir {
		chevron, folder := icTreeClosed, icTreeFolder
		if r.open {
			chevron, folder = icTreeOpen, icTreeFolderOpen
		}
		glyphs[0] = func(gtx C) D { return chevron.Layout(gtx, treeGlyph, th.MutedForeground) }
		glyphs[1] = func(gtx C) D { return folder.Layout(gtx, treeGlyph, th.MutedForeground) }
		name = name.Weight(font.Medium)
	} else {
		glyphs[0] = layout.Spacer{Width: treeGlyph, Height: treeGlyph}.Layout
		glyphs[1] = func(gtx C) D { return fileIcon(gtx, th, r.node.path) }
		if !lit {
			name = name.Muted()
		}
	}
	lead = func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(glyphs[0]), ui.Gap(1.5), layout.Rigid(glyphs[1]), ui.Gap(1.5), layout.Rigid(name.Layout))
	}
	stat = func(gtx C) D {
		d, changed := t.stats[r.node.path]
		if !changed {
			return D{}
		}
		return layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D { return diffStat(gtx, th, d.added, d.deleted) })
	}
	return lead, stat
}

// fileIcon is FileIcon.tsx: the file's devicon, or the muted file glyph at 70%
// opacity for a file it has none for.
func fileIcon(gtx C, th *ui.Theme, p string) D {
	if ic := icons.DeviconFor(p); ic != nil {
		return ic.Layout(gtx, treeGlyph, th.Foreground)
	}
	return icFileGlyph.Layout(gtx, treeGlyph, th.Over(ui.Alpha(th.MutedForeground, fileIconOpacity)))
}

const fileIconOpacity = 0.7

var icFileGlyph = icons.Lucide("file")
