package main

import (
	"image"
	"image/color"

	"gioui.org/f32"
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
	expand widget.Clickable
	add    widget.Clickable
	list   widget.List
	cards  map[string]*widget.Clickable
	tips   map[string]*ui.Tooltip
	foot   railFooter
}

// layout is `flex w-8 flex-col items-center gap-1 pb-1.5`: expand, a new
// session of the default provider, then the open sidebar's tree in one
// column (railItems). The pb-1.5 is the window's gutter around the sidebar,
// as the open one's mb-1.5 is.
func (r *sidebarRail) layout(gtx C, th *ui.Theme, s *sidebar, m *model, active string) (D, sidebarAction) {
	var act sidebarAction
	if r.expand.Clicked(gtx) {
		s.setOpen(gtx, true, &act)
	}
	if r.add.Clicked(gtx) {
		act.launch = launchRequest{kind: m.newSessionKind()}
	}
	width := gtx.Dp(ui.Space(8))
	gtx.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
	button := func(click *widget.Clickable, ic *icons.Icon, hint, glyph string) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			tip := th.Tooltip(s.root, s.hint("rail:"+hint), hint)
			tip.Side = ui.SideRight
			return tip.Layout(gtx, sideButton{click: click, icon: ic, hover: railHoverFill, morph: &s.morph, glyph: glyph}.layout(th))
		})
	}
	blocks := m.sidebarBlocks(sidebarFilter{}).blocks
	d := layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		button(&r.expand, icExpand, i18n.T("sidebar.sidebarRail.expand"), glyphToggle),
		ui.VGap(1),
		button(&r.add, icPlus, i18n.T("sidebar.sidebarRail.newSession"), glyphNew),
		ui.VGap(1),
		layout.Flexed(1, func(gtx C) D { return r.sessions(gtx, th, s, blocks, active, &act) }),
		layout.Rigid(func(gtx C) D { return r.footer(gtx, th, s, m, &act) }),
	)
	return d, act
}

// railRow is a rail row's height, a tree row's: its node and the row's
// padding, so a ring sits where it sits in the open sidebar.
var (
	railRow     = 2*sessionTreeRingR + 2*ui.Space(1.5)
	railChildR  = unit.Dp(8) // a child's node: smaller, on its mother's line
	railTrayGap = ui.Space(1.5)
)

// railKind is what a rail row draws.
type railKind int

const (
	railCard     railKind = iota
	railCheckout          // a checkout's node, the root its sessions hang from
	railHead              // a folder's or the pinned block's glyph
	railGap               // the space around a folder's tray
)

// railItem is one row of the rail's tree, the open sidebar's in one column.
type railItem struct {
	kind  railKind
	key   string // the block or checkout row it stands for, which names its node
	sv    *sessionView
	path  string // railCheckout
	facts checkoutFacts
	glyph *icons.Icon // railHead
	tint  string      // railHead: a folder's color
	title string      // railHead: the folder's name, or Pinned
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
		if folder && len(items) > 0 {
			items = append(items, railItem{kind: railGap})
		}
		first := len(items)
		switch {
		case folder:
			items = append(items, railItem{kind: railHead, key: b.key, glyph: icGroupFolder, tint: b.color, title: b.title})
		case b.kind == blockPinned:
			items = append(items, railItem{kind: railHead, key: b.key, glyph: icMenuPin, title: b.title})
		case b.under != "":
			items = append(items, railItem{kind: railCheckout, key: b.key, path: b.under, facts: b.facts[b.under]})
		}
		if !b.folded {
			items = append(items, railRows(b)...)
		}
		railLines(items[first:])
		if folder {
			for i := first; i < len(items); i++ {
				items[i].onTray, items[i].tray = true, b.color
			}
			items[first].top, items[len(items)-1].bottom = true, true
			items = append(items, railItem{kind: railGap})
		}
	}
	if n := len(items); n > 0 && items[n-1].kind == railGap {
		items = items[:n-1]
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
			items = append(items, railItem{kind: railCheckout, key: r.key, path: r.path, facts: b.facts[r.path], small: r.depth > base})
			continue
		}
		sv := r.sv
		items = append(items, railItem{kind: railCard, sv: &sv, small: r.depth > base})
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
	items := railItems(blocks)
	// The thumb floats over the list: a reserved gutter would leave 32px cells
	// a narrower column, cutting them off on the right.
	list := th.ScrollArea(&r.list)
	list.AnchorStrategy = material.Overlay
	return list.Layout(gtx, len(items), func(gtx C, i int) D {
		return r.item(gtx, th, s, items[i], active, act)
	})
}

// item draws one rail row: its tray, its line, then its node.
func (r *sidebarRail) item(gtx C, th *ui.Theme, s *sidebar, it railItem, active string, act *sidebarAction) D {
	width := gtx.Constraints.Max.X
	if it.kind == railGap {
		return D{Size: image.Pt(width, gtx.Dp(railTrayGap))}
	}
	size := image.Pt(width, gtx.Dp(railRow))
	gtx.Constraints = layout.Exact(size)
	if it.onTray {
		railTray(gtx, th, it, size)
	}
	half := gtx.Dp(sessionTreeRingR)
	if it.small {
		half = gtx.Dp(railChildR)
	}
	railLine(gtx, th, it, size, half)
	switch it.kind {
	case railHead:
		c := th.MutedForeground
		if tint, ok := cardColors[it.tint]; ok {
			c = tint
		}
		// The head's name is the one thing the rail leaves off it.
		tip := th.Tooltip(s.root, s.hint("rail:head:"+it.key), it.title)
		tip.Side = ui.SideRight
		return layout.Center.Layout(gtx, func(gtx C) D {
			return tip.Layout(gtx, func(gtx C) D {
				return s.morph.glyph(gtx, glyphNode+it.key, func(gtx C) D { return it.glyph.Layout(gtx, ui.Space(3.5), c) })
			})
		})
	case railCheckout:
		return r.checkout(gtx, th, s, it)
	}
	return r.session(gtx, th, s, it, active, act)
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
func railLine(gtx C, th *ui.Theme, it railItem, size image.Point, half int) {
	x, mid := float32(size.X)/2, float32(size.Y)/2
	var segs []linkSeg
	if it.up {
		segs = append(segs, vline(x, 0, mid-float32(half)))
	}
	if it.down {
		segs = append(segs, vline(x, mid+float32(half), float32(size.Y)))
	}
	strokeLinks(gtx, segs, size, th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha)), float32(gtx.Dp(sessionTreeStroke)))
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
			node := func(gtx C) D { return checkoutNode(gtx, th, icBranch, treeEntry{}) }
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
