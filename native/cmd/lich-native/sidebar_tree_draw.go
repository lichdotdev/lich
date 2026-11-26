package main

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// treePlace is where a row sits in its block's tree, what its line draws
// before the ring and the links it draws around it.
type treePlace struct {
	tree bool // the block is a tree: every row is indented by its level
	// hot is a foldable row under the pointer: its node shows the fold's
	// chevron in place of its glyph.
	hot bool
	// onTray is a card on a folder tray of its own color, which the tray
	// already paints at rest.
	onTray bool
	// ground is the surface under the row's node, a folder's tray or the
	// sidebar, which its ring is mixed over whatever the row paints.
	ground color.NRGBA
	sessionRowLinks
	kids int
	// descendants is every session under the card's, what its menu closes
	// with Close child sessions.
	descendants []string
	folded      bool
	mark        string
	entry       treeEntry
	// branchSaid is sessionRow's: the card leaves its branch out.
	branchSaid bool
}

// sessionRowLinks are the lines a row draws (sessionRow's fields of the same
// names).
type sessionRowLinks struct {
	depth int
	lanes []bool
	tail  bool
	stub  bool
	// sweep is a moving run's line: it draws its column only as far as it
	// has come, nil at rest.
	sweep *treeSweep
	// cut is the share of the row's own elbow not drawn, from its ring back
	// to its parent's line: a card dragged lets go of its line.
	cut float32
	// loose is a card in the air, which draws none of its lines.
	loose bool
}

func (r sessionRow) place(tree bool) treePlace {
	p := treePlace{
		tree:  tree,
		depth: r.depth, lanes: r.lanes, tail: r.tail, stub: r.stub,
		kids: r.kids, folded: r.folded, mark: r.mark, branchSaid: r.branchSaid,
	}
	if r.motion != nil {
		p.sweep, p.entry = &r.motion.sweep, r.motion.entry
	}
	return p
}

// The tree's geometry: a level's indent and the ring's radius, in the order
// a row lays them out after its left inset.
var (
	sessionTreeIndent = unit.Dp(16)
	sessionTreeRingR  = unit.Dp(11)
	sessionTreeElbow  = unit.Dp(5)
	sessionTreeStroke = unit.Dp(1.25)
	// treeLinkAlpha is how strongly the muted foreground draws the links.
	treeLinkAlpha float32 = 0.45
	rowInset              = ui.Space(1.5)
)

// treeColumn is the x of the ring centre at level k.
func treeColumn(gtx C, k int) float32 {
	return float32(gtx.Dp(rowInset) + k*gtx.Dp(sessionTreeIndent) + gtx.Dp(sessionTreeRingR))
}

// inNodeColumn lays w centred in a column as wide as a tree node, so a
// list's leading icon lines up with the sidebar's rings, its label with their
// names (DESIGN.md › the island's edge).
func inNodeColumn(w layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Dp(2 * sessionTreeRingR)
		gtx.Constraints.Max.X = gtx.Constraints.Min.X
		return layout.Center.Layout(gtx, w)
	}
}

// foldIcon is the chevron a foldable node shows in place of its glyph under
// the pointer: down while open, right while folded.
func foldIcon(open bool) *icons.Icon {
	if open {
		return icFoldOpen
	}
	return icFoldShut
}

// treeLead is what a row draws before its ring: the indent of its level.
func treeLead(p treePlace) []layout.FlexChild {
	if !p.tree || p.depth == 0 {
		return nil
	}
	return []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return D{Size: image.Pt(gtx.Dp(sessionTreeIndent)*p.depth, 0)}
	})}
}

// treeLinks draws a row's links over size: the elbow from its parent's ring,
// the lines of earlier levels that run past it, and the stub down to its
// first child. A line runs cardGap past the row so it meets the next one.
// While a run moves, the column its line runs down draws only as far as the
// line has come, and the elbow off it as far as it has turned.
func treeLinks(gtx C, th *ui.Theme, l sessionRowLinks, size image.Point) {
	if l.loose || l.depth == 0 && !l.stub && len(l.lanes) == 0 {
		return
	}
	h := float32(size.Y)
	end := h + float32(gtx.Dp(cardGap))
	whole, down, turn, own := linkSegs(gtx, l, h, end)
	c := th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha))
	width := float32(gtx.Dp(sessionTreeStroke))
	strokeLinks(gtx, whole, image.Pt(size.X, int(math.Ceil(float64(end)))), c, width)
	if len(own) > 0 {
		px, ring := treeColumn(gtx, l.depth-1), treeColumn(gtx, l.depth)-float32(gtx.Dp(sessionTreeRingR))
		reach := px - width + (ring-px+2*width)*(1-l.cut)
		strokeLinks(gtx, own, image.Pt(int(reach), size.Y), c, width)
	}
	if l.sweep != nil {
		reach := l.sweep.rows * h
		strokeLinks(gtx, down, image.Pt(size.X, int(reach)), c, width)
		turned := treeColumn(gtx, l.depth-1) + max(0, reach-(h/2-float32(gtx.Dp(sessionTreeElbow))))
		strokeLinks(gtx, turn, image.Pt(int(turned), size.Y), c, width)
	}
}

// linkSegs are a row's links of height h, running down to end: whole draws
// at once, while down (the swept column's lines) and turn (the elbow off it)
// draw as far as a moving run's line has come.
func linkSegs(gtx C, l sessionRowLinks, h, end float32) (whole, down, turn, own []linkSeg) {
	mid := h / 2
	elbow := float32(gtx.Dp(sessionTreeElbow))
	swept := -1
	if l.sweep != nil {
		swept = l.sweep.col
	}
	in := func(k int) *[]linkSeg {
		if k == swept {
			return &down
		}
		return &whole
	}
	if l.depth > 0 {
		px := treeColumn(gtx, l.depth-1)
		ring := treeColumn(gtx, l.depth) - float32(gtx.Dp(sessionTreeRingR))
		*in(l.depth - 1) = append(*in(l.depth - 1), vline(px, 0, mid-elbow))
		bend := func(p *clip.Path) {
			p.MoveTo(f32.Pt(px, mid-elbow))
			p.QuadTo(f32.Pt(px, mid), f32.Pt(px+elbow, mid))
			p.LineTo(f32.Pt(ring, mid))
		}
		switch {
		case l.depth-1 == swept:
			turn = append(turn, bend)
		case l.cut > 0:
			own = append(own, bend)
		default:
			whole = append(whole, bend)
		}
		if !l.tail {
			*in(l.depth - 1) = append(*in(l.depth - 1), vline(px, mid-elbow, end))
		}
	}
	for k, on := range l.lanes {
		if on {
			*in(k) = append(*in(k), vline(treeColumn(gtx, k), 0, end))
		}
	}
	if l.stub {
		x := treeColumn(gtx, l.depth)
		whole = append(whole, vline(x, mid+float32(gtx.Dp(sessionTreeRingR)), end))
	}
	return whole, down, turn, own
}

// linkSeg adds one stretch of a link to a path.
type linkSeg func(p *clip.Path)

func vline(x, y0, y1 float32) linkSeg {
	return func(p *clip.Path) {
		p.MoveTo(f32.Pt(x, y0))
		p.LineTo(f32.Pt(x, y1))
	}
}

// strokeLinks strokes segs as one path, clipped to box.
func strokeLinks(gtx C, segs []linkSeg, box image.Point, c color.NRGBA, width float32) {
	if len(segs) == 0 || box.X <= 0 || box.Y <= 0 {
		return
	}
	defer clip.Rect{Max: box}.Push(gtx.Ops).Pop()
	var path clip.Path
	path.Begin(gtx.Ops)
	for _, seg := range segs {
		seg(&path)
	}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: path.End(), Width: width}.Op())
}

// foldedCount is a folded row's hidden sessions and their mark.
func foldedCount(th *ui.Theme, kids int, mark string) []layout.FlexChild {
	var out []layout.FlexChild
	if dot, ok := markColor(th, blockView{collapsed: true, mark: mark}); ok {
		out = append(out, layout.Rigid(func(gtx C) D { return ui.MarkDot(gtx, dot) }), ui.Gap(1))
	}
	return append(out, layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(kids)).Muted().Mono().Layout), ui.Gap(1.5))
}

// checkoutNode is a checkout's node in the tree: its glyph in a neutral ring,
// so a link meets it as it meets a session's ring.
func checkoutNode(gtx C, th *ui.Theme, glyph *icons.Icon, entry treeEntry) D {
	side := gtx.Dp(sessionTreeRingR) * 2
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	return ui.CenteredIcon(gtx, glyph, 12, ui.Alpha(th.MutedForeground, 1-entry.glyph))
}

// editorFileNode is an editor's node: its file's icon, as the Files tree draws
// it, in the node's box and faded as checkoutNode's glyph is.
func editorFileNode(gtx C, th *ui.Theme, rel string, entry treeEntry) D {
	side := gtx.Dp(sessionTreeRingR) * 2
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	if a := 1 - entry.glyph; a < 1 {
		defer paint.PushOpacity(gtx.Ops, a).Pop()
	}
	return layout.Center.Layout(gtx, th.FileIcon(rel, 12).Layout)
}

// nodeGround is the surface under a node of b's rows: its tray in a folder,
// else the sidebar the theme is on.
func nodeGround(th *ui.Theme, b blockView) color.NRGBA {
	if b.kind == blockFolder {
		return trayColor(th, b.color)
	}
	return th.Surface
}

// onGround is th on ground, or th itself where a row names none.
func onGround(th *ui.Theme, ground color.NRGBA) *ui.Theme {
	if ground.A == 0 {
		return th
	}
	return th.On(ground)
}

// menuState is key's menu in states, made on first use.
func (s *sidebar) menuState(states map[string]*ui.MenuState, key string) *ui.MenuState {
	m, ok := states[key]
	if !ok {
		m = new(ui.MenuState)
		states[key] = m
	}
	return m
}
