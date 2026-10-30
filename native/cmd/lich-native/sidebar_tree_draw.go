package main

import (
	"image"
	"math"
	"strconv"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// treePlace is where a row sits in its block's tree, what its line draws
// before the ring and the links it draws around it.
type treePlace struct {
	tree bool // the block is a tree: every row keeps the chevron's column
	sessionRowLinks
	kids    int
	folded  bool
	mark    string
	hideGit bool
}

// sessionRowLinks are the lines a row draws (sessionRow's fields of the same
// names).
type sessionRowLinks struct {
	depth int
	lanes []bool
	tail  bool
	stub  bool
}

func (r sessionRow) place(tree bool) treePlace {
	return treePlace{
		tree:            tree,
		sessionRowLinks: sessionRowLinks{depth: r.depth, lanes: r.lanes, tail: r.tail, stub: r.stub},
		kids:            r.kids, folded: r.folded, mark: r.mark, hideGit: r.hideGit,
	}
}

// The tree's geometry: a level's indent, the chevron's column and the
// ring's radius, in the order a row lays them out after its left inset.
var (
	sessionTreeIndent  = unit.Dp(14)
	sessionTreeChevron = unit.Dp(14)
	sessionTreeRingR   = unit.Dp(9)
	sessionTreeElbow   = unit.Dp(5)
	sessionTreeStroke  = unit.Dp(1.25)
	// treeLinkAlpha is how strongly the muted foreground draws the links.
	treeLinkAlpha float32 = 0.45
	rowInset              = ui.Space(1.5)
)

// treeColumn is the x of the ring centre at level k.
func treeColumn(gtx C, k int) float32 {
	return float32(gtx.Dp(rowInset) + k*gtx.Dp(sessionTreeIndent) + gtx.Dp(sessionTreeChevron) + gtx.Dp(ui.Space(1.5)) + gtx.Dp(sessionTreeRingR))
}

// treeLead is what a row draws before its ring: the indent of its level and
// the chevron's column, the chevron itself when the row folds.
func treeLead(p treePlace, chev layout.Widget) []layout.FlexChild {
	if !p.tree {
		return nil
	}
	return []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			w := gtx.Dp(sessionTreeIndent)*p.depth + gtx.Dp(sessionTreeChevron)
			gtx.Constraints = layout.Exact(image.Pt(w, gtx.Dp(sessionTreeChevron)))
			if chev == nil {
				return D{Size: gtx.Constraints.Min}
			}
			return layout.E.Layout(gtx, chev)
		}),
		ui.Gap(1.5),
	}
}

// treeLinks draws a row's links over size: the elbow from its parent's ring,
// the lines of earlier levels that run past it, and the stub down to its
// first child. A line runs cardGap past the row so it meets the next one.
func treeLinks(gtx C, th *ui.Theme, l sessionRowLinks, size image.Point) {
	if l.depth == 0 && !l.stub && len(l.lanes) == 0 {
		return
	}
	h := float32(size.Y)
	end := h + float32(gtx.Dp(cardGap))
	mid := h / 2
	elbow := float32(gtx.Dp(sessionTreeElbow))
	var path clip.Path
	path.Begin(gtx.Ops)
	if l.depth > 0 {
		px := treeColumn(gtx, l.depth-1)
		path.MoveTo(f32.Pt(px, 0))
		path.LineTo(f32.Pt(px, mid-elbow))
		path.QuadTo(f32.Pt(px, mid), f32.Pt(px+elbow, mid))
		path.LineTo(f32.Pt(treeColumn(gtx, l.depth)-float32(gtx.Dp(sessionTreeRingR)), mid))
		if !l.tail {
			path.MoveTo(f32.Pt(px, mid-elbow))
			path.LineTo(f32.Pt(px, end))
		}
	}
	for k, on := range l.lanes {
		if on {
			path.MoveTo(f32.Pt(treeColumn(gtx, k), 0))
			path.LineTo(f32.Pt(treeColumn(gtx, k), end))
		}
	}
	if l.stub {
		x := treeColumn(gtx, l.depth)
		path.MoveTo(f32.Pt(x, mid+float32(gtx.Dp(sessionTreeRingR))))
		path.LineTo(f32.Pt(x, end))
	}
	c := th.Over(ui.Alpha(th.MutedForeground, treeLinkAlpha))
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: path.End(), Width: float32(gtx.Dp(sessionTreeStroke))}.Op())
}

// withLinks lays row out and draws its links over it.
func withLinks(gtx C, th *ui.Theme, l sessionRowLinks, row layout.Widget) D {
	d := row(gtx)
	treeLinks(gtx, th, l, d.Size)
	return d
}

// treeChevronGlyph is a mother's or a checkout's fold, a button on its own.
func treeChevronGlyph(th *ui.Theme, click *widget.Clickable, open bool) layout.Widget {
	return func(gtx C) D {
		return click.Layout(gtx, func(gtx C) D {
			c := th.MutedForeground
			if click.Hovered() {
				c = th.Foreground
			}
			return chevron(gtx, c, open)
		})
	}
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
func checkoutLine(th *ui.Theme, f checkoutFacts, p treePlace, chev layout.Widget, trailing ...layout.FlexChild) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		row := treeLead(p, chev)
		row = append(row,
			layout.Rigid(func(gtx C) D { return checkoutNode(gtx, th) }),
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

// checkoutNode is a checkout's node in the tree: the branch glyph in a
// neutral ring, so a link meets it as it meets a session's ring.
func checkoutNode(gtx C, th *ui.Theme) D {
	side := gtx.Dp(sessionTreeRingR) * 2
	stroke := float32(gtx.Dp(sessionTreeStroke))
	center := f32.Pt(float32(side)/2, float32(side)/2)
	ui.StrokeArc(gtx, center, float32(side)/2-stroke/2, stroke, 0, 2*math.Pi, th.Over(ui.Alpha(th.MutedForeground, neutralRingAlpha)))
	gtx.Constraints = layout.Exact(image.Pt(side, side))
	return centeredIcon(gtx, icBranch, 10, th.MutedForeground)
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
	return tooltip.Layout(gtx, func(gtx C) D { return withLinks(gtx, th, p.sessionRowLinks, checkoutLine(th, f, p, nil)) })
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
	chev := func(gtx C) D { return chevron(gtx, th.MutedForeground, !b.collapsed) }
	line := checkoutLine(th, facts, p, chev, count...)
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
