package main

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/lichdotdev/lich/native/i18n"
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
	sessionRowLinks
	kids    int
	folded  bool
	mark    string
	hideGit bool
	entry   treeEntry
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
}

func (r sessionRow) place(tree bool) treePlace {
	p := treePlace{
		tree:            tree,
		sessionRowLinks: sessionRowLinks{depth: r.depth, lanes: r.lanes, tail: r.tail, stub: r.stub},
		kids:            r.kids, folded: r.folded, mark: r.mark, hideGit: r.hideGit,
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
	if l.depth == 0 && !l.stub && len(l.lanes) == 0 {
		return
	}
	h := float32(size.Y)
	end := h + float32(gtx.Dp(cardGap))
	whole, down, turn := linkSegs(gtx, l, h, end)
	c := th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha))
	width := float32(gtx.Dp(sessionTreeStroke))
	strokeLinks(gtx, whole, image.Pt(size.X, int(math.Ceil(float64(end)))), c, width)
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
func linkSegs(gtx C, l sessionRowLinks, h, end float32) (whole, down, turn []linkSeg) {
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
		if l.depth-1 == swept {
			turn = append(turn, bend)
		} else {
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
	return whole, down, turn
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

// withLinks lays row out and draws its links over it.
func withLinks(gtx C, th *ui.Theme, l sessionRowLinks, row layout.Widget) D {
	d := row(gtx)
	treeLinks(gtx, th, l, d.Size)
	return d
}

// foldedCount is a folded row's hidden sessions and their mark.
func foldedCount(th *ui.Theme, kids int, mark string) []layout.FlexChild {
	var out []layout.FlexChild
	if dot, ok := markColor(th, blockView{collapsed: true, mark: mark}); ok {
		out = append(out, layout.Rigid(func(gtx C) D { return markDot(gtx, dot) }), ui.Gap(1))
	}
	return append(out, layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(kids)).Muted().Mono().Layout), ui.Gap(1.5))
}

// checkoutLine is a checkout row's line: the branch glyph where a session's
// ring sits, the branch, its pull request and its changes.
func checkoutLine(th *ui.Theme, f checkoutFacts, p treePlace, open bool, trailing ...layout.FlexChild) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		glyph := icBranch
		if p.hot {
			glyph = foldIcon(open)
		}
		row := treeLead(p)
		row = append(row,
			layout.Rigid(func(gtx C) D { return checkoutNode(gtx, th, glyph, p.entry) }),
			ui.Gap(1.5),
			layout.Flexed(1, th.Text(ui.TextSM, f.label).Muted().Weight(font.Medium).Layout),
		)
		row = append(row, trailing...)
		if f.pr != nil {
			row = append(row, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, "#"+strconv.Itoa(f.pr.Number)).Muted().Layout))
		}
		if f.git.Files > 0 {
			row = append(row, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return diffStat(gtx, th, f.git.Added, f.git.Deleted) }))
		}
		return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: rowInset, Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
}

// checkoutNode is a checkout's node in the tree: its glyph in a neutral ring,
// so a link meets it as it meets a session's ring.
func checkoutNode(gtx C, th *ui.Theme, glyph *icons.Icon, entry treeEntry) D {
	side := gtx.Dp(sessionTreeRingR) * 2
	ringArc(gtx, side, entry, th.Over(ui.Alpha(th.MutedForeground, neutralRingAlpha)))
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	return centeredIcon(gtx, glyph, 12, ui.Alpha(th.MutedForeground, 1-entry.body))
}

// ringArc draws a node's ring in a box of side, as much of it as has come in:
// from its left, where the link meets it, round to the left again.
func ringArc(gtx C, side int, entry treeEntry, c color.NRGBA) {
	sweep := 2 * math.Pi * (1 - entry.ring)
	if sweep <= 0 {
		return
	}
	stroke := float32(gtx.Dp(sessionTreeStroke))
	center := f32.Pt(float32(side)/2, float32(side)/2)
	ui.StrokeArc(gtx, center, float32(side)/2-stroke/2, stroke, math.Pi, float32(sweep), c)
}

// checkoutTooltip is what a checkout row's line cuts: the branch in full, the
// checkout's path, its standing against the base, and its pull request's
// title.
func checkoutTooltip(th *ui.Theme, path string, f checkoutFacts) layout.Widget {
	th = th.On(th.Popover)
	sections := []layout.Widget{}
	if f.git.Branch != "" {
		branch := th.Text(ui.TextXS, f.git.Branch).Weight(font.Medium)
		sections = append(sections, func(gtx C) D { return iconLine(gtx, icBranch, branch.Muted(), layout.Flexed(1, para(branch).Layout)) })
	}
	sections = append(sections, para(th.Text(ui.TextXS, path).Muted().Mono()).Layout)
	sections = append(sections, baseSections(th, f.base)...)
	if f.pr != nil && f.pr.Title != "" {
		title := th.Text(ui.TextXS, f.pr.Title)
		sections = append(sections, func(gtx C) D { return iconLine(gtx, icPR, title.Muted(), layout.Flexed(1, para(title).Layout)) })
	}
	return func(gtx C) D { return stack(gtx, 1.5, sections) }
}

// nestedCheckout is a checkout row inside a mother's group, the checkout her
// children under it run in.
func (s *sidebar) nestedCheckout(gtx C, th *ui.Theme, r sessionRow, f checkoutFacts, tree bool) D {
	tip, ok := s.tips[r.key]
	if !ok {
		tip = new(ui.Tooltip)
		s.tips[r.key] = tip
	}
	tooltip := th.TooltipCard(s.root, tip, checkoutTooltip(th, r.path, f))
	tooltip.Side = ui.SideRight
	p := r.place(tree)
	return tooltip.Layout(gtx, func(gtx C) D { return withLinks(gtx, th, p.sessionRowLinks, checkoutLine(th, f, p, true)) })
}

// checkoutHeader is a checkout block's header as the root row of its tree:
// the fold, the checkout's line, its tooltip, and a menu that opens a session
// there or files the whole block.
func (s *sidebar) checkoutHeader(gtx C, th *ui.Theme, b blockView, st *blockState, f blockFrame) D {
	facts := b.facts[b.under]
	p := treePlace{tree: true, sessionRowLinks: sessionRowLinks{stub: !b.collapsed && len(b.sessions) > 0}}
	var count []layout.FlexChild
	if b.collapsed {
		count = foldedCount(th, len(b.sessions), b.mark)
	}
	tip, ok := s.tips["__checkout__:"+b.key]
	if !ok {
		tip = new(ui.Tooltip)
		s.tips["__checkout__:"+b.key] = tip
	}
	tooltip := th.TooltipCard(s.root, tip, checkoutTooltip(th, b.under, facts))
	tooltip.Side = ui.SideRight
	hooks := blockMenuHooks{rename: func() {}, newFolder: s.askFolderName}
	entries := func() []ui.MenuEntry {
		menu := []ui.MenuEntry{{Kind: ui.MenuSubTrigger, Label: i18n.T("sidebar.checkoutRow.newSessionHere"), Icon: icPlus, Sub: blockLaunch(b, s.launchProviders, f.act)}}
		return append(menu, blockOptions(b, f.view.folders, f.filtering, f.act, hooks)...)
	}
	p.hot = st.more.Hovered()
	line := checkoutLine(th, facts, p, !b.collapsed, count...)
	row := func(gtx C) D {
		return th.ContextMenuLazy(s.root, &st.options, entries).Layout(gtx, func(gtx C) D {
			return tooltip.Layout(gtx, func(gtx C) D {
				return st.more.Layout(gtx, func(gtx C) D {
					fill := cardFill(th, "", false, st.more.Hovered())
					return layout.Background{}.Layout(gtx,
						func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
						func(gtx C) D { return withLinks(gtx, th, p.sessionRowLinks, line) },
					)
				})
			})
		})
	}
	if st.more.Clicked(gtx) && !f.headerDragEnded {
		f.act.fold = b.key
	}
	if f.filtering {
		return row(gtx)
	}
	return s.sort.Item(gtx, b.key, row, nil)
}
