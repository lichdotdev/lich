package filetree

import (
	"image"

	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var (
	icTreeOpen       = icons.Lucide("chevron-down")
	icTreeClosed     = icons.Lucide("chevron-right")
	icTreeFolder     = icons.Lucide("folder")
	icTreeFolderOpen = icons.Lucide("folder-open")
)

// Tree geometry: rows indented 0.75rem a level from a 0.5rem base, 0.875rem
// glyphs.
var (
	Indent     = ui.Space(3)
	IndentBase = ui.Space(2)
	Glyph      = ui.Space(3.5)
)

// Rows is the tree on screen: its list, its rows' clickables and the tree
// the caller built, with the line counts by path Stats keeps for it.
type Rows struct {
	List   widget.List
	clicks map[string]*treeClickable

	// The tree scrolls sideways once a row is wider than the column: widest
	// is the widest row the last frame laid out, offsetX how far it scrolled.
	hscroll gesture.Scroll
	hbar    widget.Scrollbar
	vbar    widget.Scrollbar
	widest  int
	measure int
	offsetX int

	Tree  []*Node
	Stats map[string]Delta
}

// treeClickable is a row's clickable and the node it was last drawn for,
// which clicked answers with.
type treeClickable struct {
	click widget.Clickable
	node  *Node
}

func (t *Rows) click(node *Node) *widget.Clickable {
	if t.clicks == nil {
		t.clicks = map[string]*treeClickable{}
	}
	c, ok := t.clicks[node.Path]
	if !ok {
		c = new(treeClickable)
		t.clicks[node.Path] = c
	}
	c.node = node
	return &c.click
}

// Clicked is every row clicked since the last frame. Callers act on it
// before they read which rows show: a click handled after the rows were read
// draws the frame it arrived in from the old tree, and nothing redraws until
// the next event.
func (t *Rows) Clicked(gtx C) []*Node {
	var nodes []*Node
	for _, c := range t.clicks {
		for c.click.Clicked(gtx) {
			nodes = append(nodes, c.node)
		}
	}
	return nodes
}

// Layout is the tree's rows, scrolling down and, once a row is wider
// than the column, sideways, with its own bar along the bottom. While the
// rows overflow downward they stop short of the vertical bar, as the
// sidebar's Occupy keeps its cards off it.
func (t *Rows) Layout(gtx C, th *ui.Theme, rows []Row, selected string) D {
	view := gtx.Constraints.Max
	style := th.ScrollArea(&t.List).ScrollbarStyle
	overflowsY := t.List.Position.Length > view.Y && len(rows) > 0
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
		t.List.List.ScrollBy(d * float32(len(rows)))
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
		return t.List.List.Layout(gtx, len(rows), func(gtx C, i int) D {
			return t.row(gtx, th, rows[i], selected)
		})
	})
	stack.Pop()
	rowsClip.Pop()
	t.widest = t.measure
	t.bars(gtx, style, view, content, width, len(rows), overflowsY)
	return D{Size: view}
}

// bars draws the sideways bar along the bottom and the vertical one on the
// right, each only once its axis overflows.
func (t *Rows) bars(gtx C, style material.ScrollbarStyle, view, content image.Point, width, rows int, overflowsY bool) {
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
		pos := t.List.Position
		row := pos.Length / rows
		start := float32(pos.First*row+pos.Offset) / float32(pos.Length)
		layout.E.Layout(gtx, func(gtx C) D {
			return bar(gtx, &t.vbar, layout.Vertical, start, start+float32(view.Y)/float32(pos.Length))
		})
	}
}

// measureRow notes the row's own width, its two ends side by side inside
// inset, which the next frame scrolls the tree by. The ends are measured
// apart: the spring between them would take all the room it is given.
func (t *Rows) measureRow(gtx C, inset layout.Inset, lead, stat layout.Widget) {
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
func (t *Rows) row(gtx C, th *ui.Theme, r Row, selected string) D {
	click := t.click(r.Node)
	return click.Layout(gtx, func(gtx C) D {
		// Read inside Layout, which takes the pointer events first: read
		// before it, the hover lags one event behind.
		lit := click.Hovered() || (!r.Node.Dir && r.Node.Path == selected)
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
				indent := IndentBase + Indent*unit.Dp(r.Depth)
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
func (t *Rows) rowParts(th *ui.Theme, r Row, lit bool) (lead, stat layout.Widget) {
	name := th.Text(ui.TextXS, r.Node.Name).Mono()
	name.MaxLines = 1
	var glyphs [2]layout.Widget
	if r.Node.Dir {
		chevron, folder := icTreeClosed, icTreeFolder
		if r.Open {
			chevron, folder = icTreeOpen, icTreeFolderOpen
		}
		glyphs[0] = func(gtx C) D { return chevron.Layout(gtx, Glyph, th.MutedForeground) }
		glyphs[1] = func(gtx C) D { return folder.Layout(gtx, Glyph, th.MutedForeground) }
		name = name.Weight(font.Medium)
	} else {
		glyphs[0] = layout.Spacer{Width: Glyph, Height: Glyph}.Layout
		glyphs[1] = func(gtx C) D { return FileIcon(gtx, th, r.Node.Path) }
		if !lit {
			name = name.Muted()
		}
	}
	lead = func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(glyphs[0]), ui.Gap(1.5), layout.Rigid(glyphs[1]), ui.Gap(1.5), layout.Rigid(name.Layout))
	}
	stat = func(gtx C) D {
		d, changed := t.Stats[r.Node.Path]
		if !changed {
			return D{}
		}
		return layout.Inset{Left: ui.Space(2)}.Layout(gtx, func(gtx C) D { return th.DiffStat(d.Added, d.Deleted).Layout(gtx) })
	}
	return lead, stat
}

// FileIcon draws the file's devicon, or the muted file glyph at 70%
// opacity for a file it has none for.
func FileIcon(gtx C, th *ui.Theme, p string) D {
	if ic := icons.DeviconFor(p); ic != nil {
		return ic.Layout(gtx, Glyph, th.Foreground)
	}
	return icFileGlyph.Layout(gtx, Glyph, th.Over(ui.Alpha(th.MutedForeground, fileIconOpacity)))
}

const fileIconOpacity = 0.7

var icFileGlyph = icons.Lucide("file")
