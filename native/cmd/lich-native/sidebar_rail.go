package main

import (
	"image"
	"image/color"
	"maps"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// SidebarRail's buttons hover bg-accent/60.
const railHoverFill = 0.6

var icExpand = icons.Lucide("panel-left")

// sidebarRail is SidebarRail: the collapsed sidebar, the same tree in the
// same order reduced to its nodes in one column. It selects and it creates,
// and that is all; everything else is the open sidebar's.
type sidebarRail struct {
	// motion is the rail's tree motion, as a block's is in the open sidebar,
	// and last the rows it last drew, which a row leaving is drawn from.
	motion treeMotion
	last   map[string]railItem
	expand widget.Clickable
	add    widget.Clickable
	list   widget.List
	cards  map[string]*widget.Clickable
	tips   map[string]*ui.Tooltip
	menus  map[string]*ui.MenuState
	// view is the frame's sidebar view, whose blocks and folders a node's
	// menu reads.
	view sidebarView
	foot railFooter
}

// layout draws the rail as an island like the open sidebar, its content on
// the island's 8px edge: expand where the open header's collapse sits, the
// open sidebar's tree in one column (railItems), then the footer.
func (r *sidebarRail) layout(gtx C, th *ui.Theme, s *sidebar, m *model, active string) (D, sidebarAction) {
	var act sidebarAction
	if r.expand.Clicked(gtx) {
		s.setOpen(gtx, true, &act)
	}
	if r.add.Clicked(gtx) {
		act.launch = launchRequest{kind: m.newSessionKind()}
	}
	size := image.Pt(gtx.Dp(railWidth), gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)
	ui.Fill(gtx, th.Sidebar, ui.RadiusLG)
	th = th.On(th.Sidebar)
	r.view = m.sidebarBlocks(sidebarFilter{})
	blocks := r.view.blocks
	layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			// Expand sits in a row of the open header's height, so it lands
			// where collapse sits in the open sidebar.
			layout.Rigid(func(gtx C) D {
				gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(railHeaderRow)))
				return layout.Center.Layout(gtx, r.button(th, s, &r.expand, icExpand, i18n.T("sidebar.sidebarRail.expand"), glyphToggle))
			}),
			ui.VGap(2),
			layout.Flexed(1, func(gtx C) D { return r.sessions(gtx, th, s, blocks, active, &act) }),
			layout.Rigid(func(gtx C) D { return r.footer(gtx, th, s, m, &act) }),
		)
	})
	return D{Size: size}, act
}

// button is one of the rail's buttons under its hint.
func (r *sidebarRail) button(th *ui.Theme, s *sidebar, click *widget.Clickable, ic *icons.Icon, hint, glyph string) layout.Widget {
	return func(gtx C) D {
		tip := th.Tooltip(s.root, s.hint("rail:"+hint), hint)
		tip.Side = ui.SideRight
		return tip.Layout(gtx, sideButton{click: click, icon: ic, hover: railHoverFill, morph: &s.morph, glyph: glyph}.layout(th))
	}
}

// railRow is a rail row's height, a tree row's: its node and the row's
// padding, so a ring sits where it sits in the open sidebar.
var (
	// railWidth is the rail's island: a column as wide as a tree row is tall
	// inside the island's 8px edge, so its nodes centre where the open
	// sidebar's do (rowInset + sessionTreeRingR from the edge).
	railWidth = railRow + 2*ui.Space(2)
	// railHeaderRow is the open sidebar's header row (labelFace).
	railHeaderRow = ui.Space(9)
	railRow       = 2*sessionTreeRingR + 2*ui.Space(1.5)
	railChildR    = unit.Dp(8)    // a child's node: smaller, on its mother's line
	railBlockGap  = ui.Space(1.5) // the gap between blocks, blockList's
)

// railKind is what a rail row draws.
type railKind int

const (
	railCard     railKind = iota
	railCheckout          // a checkout's node, the root its sessions hang from
	railHead              // a folder's or the pinned block's glyph
	railGap               // the space between two blocks
)

// railItem is one row of the rail's tree, the open sidebar's in one column.
type railItem struct {
	kind railKind
	// id is the row among the rail's rows, and depth its level in its tree:
	// what the rail's motion reads off them frame to frame.
	id     string
	depth  int
	motion *rowMotion
	key    string // the block or checkout row it stands for, which names its node
	block  string // the block the row is in, whose menu its node opens
	kids   int    // railCard: the sessions under a mother
	folded bool   // railCard: a mother folded over them
	sv     *sessionView
	path   string // railCheckout
	facts  checkoutFacts
	glyph  *icons.Icon // railHead
	tint   string      // railHead: a folder's color
	title  string      // railHead: the folder's name, or Pinned
	// small is a child's node; up and down are the line to the node above
	// and below in the same tree.
	small, up, down bool
	// tray is the folder's color for a row on its tray (tray set), with top
	// and bottom its ends.
	tray        string
	onTray      bool
	top, bottom bool
}

// railItems lays the blocks out as the rail's rows: each block's head, then
// its rows as the open sidebar's tree has them, a mother's folded children
// left out with her, one line through each tree.
func railItems(blocks []blockView) []railItem {
	var items []railItem
	for _, b := range blocks {
		folder := b.kind == blockFolder
		// Blocks stand apart as the open sidebar's do, so every node keeps
		// its height across the morph.
		if len(items) > 0 {
			items = append(items, railItem{kind: railGap, id: "gap:" + b.key, block: b.key})
		}
		first := len(items)
		switch {
		case folder:
			items = append(items, railItem{kind: railHead, id: "head:" + b.key, key: b.key, glyph: icGroupFolder, tint: b.color, title: b.title})
		case b.kind == blockPinned:
			items = append(items, railItem{kind: railHead, id: "head:" + b.key, key: b.key, glyph: icMenuPin, title: b.title})
		case b.under != "":
			items = append(items, railItem{kind: railCheckout, id: "checkout:" + b.key, key: b.key, path: b.under, facts: b.facts[b.under]})
		}
		if !b.folded {
			items = append(items, railRows(b)...)
		}
		for i := first; i < len(items); i++ {
			items[i].block = b.key
		}
		railLines(items[first:])
		if folder {
			for i := first; i < len(items); i++ {
				items[i].onTray, items[i].tray = true, b.color
			}
			items[first].top, items[len(items)-1].bottom = true, true
		}
	}
	return items
}

// railRows is a block's rows past its head.
func railRows(b blockView) []railItem {
	base := 0
	if b.under != "" || b.byCheckout {
		base = 1
	}
	var items []railItem
	for _, r := range blockRows(b, b.sessions) {
		if r.kind == rowNestedCheckout {
			items = append(items, railItem{kind: railCheckout, id: r.key, depth: r.depth, key: r.key, path: r.path, facts: b.facts[r.path], small: r.depth > base})
			continue
		}
		sv := r.sv
		items = append(items, railItem{kind: railCard, id: r.key, depth: r.depth, sv: &sv, small: r.depth > base, kids: r.kids, folded: r.folded})
	}
	return items
}

// railLines links each node to the next in its tree. A checkout a folder
// gathers starts a tree of its own, and a head is no node.
func railLines(items []railItem) {
	for i := 1; i < len(items); i++ {
		prev, cur := &items[i-1], &items[i]
		root := cur.kind == railCheckout && !cur.small
		if prev.kind == railHead || root {
			continue
		}
		prev.down, cur.up = true, true
	}
}

func (r *sidebarRail) sessions(gtx C, th *ui.Theme, s *sidebar, blocks []blockView, active string, act *sidebarAction) D {
	items := r.move(gtx, railItems(blocks))
	// The thumb floats over the list: a reserved gutter would leave 32px cells
	// a narrower column, cutting them off on the right.
	list := th.ScrollArea(&r.list)
	list.AnchorStrategy = material.Overlay
	return list.Layout(gtx, len(items), func(gtx C, i int) D {
		return r.item(gtx, th, s, items[i], active, act)
	})
}

// move runs the rail's rows through the tree's motion: rows that came in
// open, and rows that left are put back while they close, each with its
// share of its run.
func (r *sidebarRail) move(gtx C, items []railItem) []railItem {
	byID := make(map[string]railItem, len(items)+len(r.last))
	maps.Copy(byID, r.last)
	rows := make([]sessionRow, len(items))
	for i, it := range items {
		rows[i] = sessionRow{key: it.id, depth: it.depth}
		byID[it.id] = it
	}
	if r.motion.update(gtx.Now, rows) {
		gtx.Execute(op.InvalidateCmd{})
	}
	placed := r.motion.place(gtx.Now, rows)
	out := make([]railItem, 0, len(placed))
	for _, row := range placed {
		if it, ok := byID[row.key]; ok {
			it.motion = row.motion
			out = append(out, it)
		}
	}
	r.last = make(map[string]railItem, len(items))
	for _, it := range items {
		r.last[it.id] = it
	}
	for _, run := range r.motion.runs {
		for _, row := range run.rows {
			if it, ok := byID[row.key]; ok {
				r.last[row.key] = it
			}
		}
	}
	return out
}

// forget drops the rail's motion while the open sidebar is drawn: what
// changed meanwhile is not news to the rail.
func (r *sidebarRail) forget() { r.motion, r.last = treeMotion{}, nil }

// item draws one rail row in its motion, if it has one.
func (r *sidebarRail) item(gtx C, th *ui.Theme, s *sidebar, it railItem, active string, act *sidebarAction) D {
	if it.motion == nil {
		return r.row(gtx, th, s, it, active, act)
	}
	return ui.Reveal(gtx.Disabled(), it.motion.height, func(gtx C) D { return r.row(gtx, th, s, it, active, act) })
}

// row draws one rail row: its tray, its node, then its line.
func (r *sidebarRail) row(gtx C, th *ui.Theme, s *sidebar, it railItem, active string, act *sidebarAction) D {
	width := gtx.Constraints.Max.X
	if it.kind == railGap {
		return D{Size: image.Pt(width, gtx.Dp(railBlockGap))}
	}
	size := image.Pt(width, gtx.Dp(railRow))
	gtx.Constraints = layout.Exact(size)
	if it.onTray {
		railTray(gtx, th, it, size)
	}
	d := r.node(gtx, th, s, it, active, act)
	// The line goes over the node's square, as the open sidebar's links go
	// over a lit row, stopping at the ring.
	half := gtx.Dp(sessionTreeRingR)
	if it.small {
		half = gtx.Dp(railChildR)
	}
	railLine(gtx, th, it, size, half)
	return d
}

// node draws a row's node: a head's glyph, a checkout's ring, or a session.
func (r *sidebarRail) node(gtx C, th *ui.Theme, s *sidebar, it railItem, active string, act *sidebarAction) D {
	switch it.kind {
	case railHead:
		c := headGlyphColor(th, it.tint)
		// The head's name is the one thing the rail leaves off it, in the
		// card every other node's tooltip is.
		state, ok := r.tips["head:"+it.key]
		if !ok {
			state = new(ui.Tooltip)
			r.tips["head:"+it.key] = state
		}
		pop := th.On(th.Popover)
		tip := th.TooltipCard(s.root, state, para(pop.Text(ui.TextXS, it.title).Weight(font.Medium)).Layout)
		tip.Side = ui.SideRight
		return layout.Center.Layout(gtx, func(gtx C) D {
			return r.menu(gtx, th, s, it, act).Layout(gtx, func(gtx C) D {
				return tip.Layout(gtx, func(gtx C) D {
					// The open header's size, so the glyph flies without resizing.
					faded := ui.Alpha(c, 1-it.entry().glyph)
					return s.morph.glyph(gtx, glyphNode+it.key, func(gtx C) D { return it.glyph.Layout(gtx, ui.Space(3), faded) })
				})
			})
		})
	case railCheckout:
		return r.menu(gtx, th, s, it, act).Layout(gtx, func(gtx C) D { return r.checkout(gtx, th, s, it) })
	}
	return r.menu(gtx, th, s, it, act).Layout(gtx, func(gtx C) D { return r.session(gtx, th, s, it, active, act) })
}

// railGround is the surface under a row's node: its folder's tray, or the
// rail.
func railGround(th *ui.Theme, it railItem) color.NRGBA {
	if it.onTray {
		return trayColor(th, it.tray)
	}
	return th.Surface
}

// railTray paints a folder's tray behind a row, rounded at its ends.
func railTray(gtx C, th *ui.Theme, it railItem, size image.Point) {
	rr := gtx.Dp(ui.RadiusLG)
	shape := clip.RRect{Rect: image.Rectangle{Max: size}}
	if it.top {
		shape.NW, shape.NE = rr, rr
	}
	if it.bottom {
		shape.SW, shape.SE = rr, rr
	}
	paint.FillShape(gtx.Ops, trayColor(th, it.tray), shape.Op(gtx.Ops))
}

// railLine draws the line through a row's node: up to the node above and
// down to the next, stopping at the node's edge (half from its centre).
// A moving row draws the line down to it only as far as it has come.
func railLine(gtx C, th *ui.Theme, it railItem, size image.Point, half int) {
	x, mid := float32(size.X)/2, float32(size.Y)/2
	c, width := th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha)), float32(gtx.Dp(sessionTreeStroke))
	if it.up {
		reach := size
		if it.motion != nil {
			reach.Y = int(clamp01(it.motion.sweep.rows) * float32(size.Y))
		}
		strokeLinks(gtx, []linkSeg{vline(x, 0, mid-float32(half))}, reach, c, width)
	}
	if it.down {
		strokeLinks(gtx, []linkSeg{vline(x, mid+float32(half), float32(size.Y))}, size, c, width)
	}
}

// entry is how much of the row's node is still to come in, 0 at rest.
func (it railItem) entry() treeEntry {
	if it.motion == nil {
		return treeEntry{}
	}
	return it.motion.entry
}

// railSmall draws w, a 22px node, at a child's size around its centre.
func railSmall(gtx C, small bool, w layout.Widget) D {
	if !small {
		return w(gtx)
	}
	side := float32(gtx.Dp(2 * sessionTreeRingR))
	k := float32(gtx.Dp(railChildR)) * 2 / side
	defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(side/2, side/2), f32.Pt(k, k))).Push(gtx.Ops).Pop()
	return w(gtx)
}

// checkout is a checkout's node, under the checkout's tooltip.
func (r *sidebarRail) checkout(gtx C, th *ui.Theme, s *sidebar, it railItem) D {
	key := "rail:checkout:" + it.path
	tip, ok := r.tips[key]
	if !ok {
		tip = new(ui.Tooltip)
		r.tips[key] = tip
	}
	tooltip := th.TooltipCard(s.root, tip, checkoutTooltip(th, it.path, it.facts))
	tooltip.Side = ui.SideRight
	return layout.Center.Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D {
			node := func(gtx C) D { return checkoutNode(gtx, onGround(th, railGround(th, it)), icBranch, it.entry()) }
			if it.small {
				// A child's node is drawn smaller than it flies: it crossfades.
				return railSmall(gtx, true, node)
			}
			return s.morph.glyph(gtx, glyphNode+it.key, node)
		})
	})
}

// session is RailSession: `size-8 rounded-md` around the card's own status
// ring, lit while active, under the card's own tooltip. A child's ring is a
// child's size.
func (r *sidebarRail) session(gtx C, th *ui.Theme, s *sidebar, it railItem, active string, act *sidebarAction) D {
	sv := it.sv
	sv.place.tree = true
	sv.place.ground = railGround(th, it)
	sv.place.entry = it.entry()
	click := clickable(r.cards, sv.ID)
	if click.Clicked(gtx) {
		act.activate = sv.ID
	}
	tip, ok := r.tips[sv.ID]
	if !ok {
		tip = new(ui.Tooltip)
		r.tips[sv.ID] = tip
	}
	tooltip := th.TooltipCard(s.root, tip, sessionTooltip(th, sv))
	tooltip.Side = ui.SideRight
	return layout.Center.Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D {
			return click.Layout(gtx, func(gtx C) D {
				var bg color.NRGBA
				switch {
				case sv.ID == active:
					bg = th.Over(th.Accent)
				case click.Hovered():
					bg = th.Over(ui.Alpha(th.Accent, railHoverFill))
				}
				return squareFace(gtx, bg, func(gtx C) D {
					if it.small {
						return railSmall(gtx, true, func(gtx C) D { return s.statusGlyph(gtx, th, sv) })
					}
					return s.statusRing(gtx, th, sv)
				})
			})
		})
	})
}

// railHairline is `my-1 h-px w-5 bg-border`, over the rail's footer.
func railHairline(gtx C, th *ui.Theme) D {
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(ui.Space(5)), gtx.Dp(1)))
		return ui.Fill(gtx, th.Over(th.Border), 0)
	})
}
