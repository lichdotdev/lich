package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// The project tab, from the approved mockup: browser tabs sized like
// Chromium's, a fixed width that shrinks evenly to a floor before the strip
// scrolls.
var (
	tabWidthMax    = ui.Space(49)   // 196px
	tabWidthMin    = ui.Space(29.5) // 118px
	tabHeight      = ui.Space(9.5)  // 38px, from 6px under the band's top
	tabTop         = ui.Space(1.5)
	tabRadius      = ui.RadiusLG // its top corners, and the fillets into the island
	tabPadLeft     = ui.Space(3.5)
	tabPadRight    = ui.Space(2.5)
	tabGlyph       = ui.Space(3.5)
	tabSeparatorIn = ui.Space(2.75) // the hairline between tabs stops 11px short of each edge
)

const (
	tabTextSize    = 13
	tabTextLeading = 20
	// tabHoverFill is the mockup's --hover-faint, the foreground at 4%.
	tabHoverFill = 0.04
	// tabDetachedFill is the active tab that sits over the gap between two
	// islands and so grows out of neither: a raised fill instead of a merge.
	tabDetachedFill = 0.08
	// tabSpinPeriod is animate-spin's one turn a second.
	tabSpinPeriod = time.Second
	// kappa places a cubic's controls so it draws a quarter circle.
	kappa = 0.5523
)

var (
	icTabClose   = icons.Lucide("x")
	icTabLoader  = icons.Lucide("loader-circle")
	icTabWaiting = icons.Lucide("bell")
	icTabDone    = icons.Lucide("check")
)

type tabState struct {
	click widget.Clickable
	close widget.Clickable
	tip   ui.Tooltip
	menu  ui.MenuState
}

func (t *topbar) tabState(id string) *tabState {
	st, ok := t.tabs[id]
	if !ok {
		st = new(tabState)
		t.tabs[id] = st
	}
	return st
}

// tabWidth is each tab's width: the widest while count of them fit in
// room, else room shared evenly, never under the narrowest, past which the
// strip scrolls.
func tabWidth(room, count, widest, narrowest int) int {
	if count == 0 {
		return widest
	}
	return min(widest, max(narrowest, room/count))
}

// strip is the scrolling row of tabs with the open-project button right after
// the last one, and the fillets that merge the active tab into the island.
func (t *topbar) strip(gtx C, th *ui.Theme, f topbarFrame, act *topbarAction) D {
	// Over a tab's × the pointer keeps the plain arrow, not the drag grip's.
	t.sort.Grab = func(id string) bool { return !t.tabState(id).close.Hovered() }
	before := t.sort.Dragging()
	if d, ok := t.sort.Update(gtx); ok && d.ID != d.Over {
		act.drop = d
	}
	dragEnded := before != "" && t.sort.Dragging() == ""
	plus := gtx.Dp(barButton) + gtx.Dp(barGap)
	w := tabWidth(gtx.Constraints.Max.X-plus, len(f.tabs), gtx.Dp(tabWidthMax), gtx.Dp(tabWidthMin))
	t.tabW = w
	ids := make([]string, len(f.tabs))
	for i, tab := range f.tabs {
		ids[i] = tab.ID
	}
	size := gtx.Constraints.Max
	clipped := clip.Rect{Max: size}.Push(gtx.Ops)
	t.list.Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Top: tabTop}.Layout(gtx, func(gtx C) D {
					return t.sort.List(gtx, ui.SortList{Horizontal: true, Within: true}, ids, func(gtx C, i int) D {
						return t.tab(gtx, th, f, i, w, dragEnded, act)
					})
				})
			}),
			ui.Gap(1),
			// Centred on the band, which puts the "+" on the tabs' label line:
			// it sits a gutter above the middle of a tab, as the tab sits
			// tabTop below the band's top.
			layout.Rigid(func(gtx C) D { return t.openButton(gtx, th, act) }),
		)
	})
	t.fillets(gtx, f, w)
	clipped.Pop()
	return D{Size: size}
}

func (t *topbar) tab(gtx C, th *ui.Theme, f topbarFrame, i, w int, dragEnded bool, act *topbarAction) D {
	tab := f.tabs[i]
	st := t.tabState(tab.ID)
	closed, clicked := st.close.Clicked(gtx), st.click.Clicked(gtx)
	switch {
	case closed:
		act.ask = tab.ID
	case clicked && !dragEnded && tab.ID != f.active:
		act.show = tab.ID
	}
	gtx.Constraints = layout.Exact(image.Pt(w, gtx.Dp(tabHeight)))
	// ponytail: the mockup's Rename and Pin join the menu when the native
	// window has them.
	entries := []ui.MenuEntry{
		{Label: projectSettings, Icon: icSettings, OnClick: func() { act.projectSettings = tab.ID }},
		{Kind: ui.MenuSeparator},
		{Label: closeProjectItem, Icon: icMenuClose, Variant: ui.VariantDestructive, OnClick: func() { act.ask = tab.ID }},
	}
	return t.sort.Item(gtx, tab.ID, func(gtx C) D {
		return th.ContextMenu(t.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
			tip := th.TooltipCard(t.root, &st.tip, tabTooltip(th.On(th.Popover), tab))
			tip.Side = ui.SideBottom
			return tip.Layout(gtx, func(gtx C) D {
				return st.click.Layout(gtx, func(gtx C) D { return t.tabFace(gtx, th, f, i, st) })
			})
		})
	}, nil)
}

// tabFace draws a tab: the island's color when active, a faint fill when
// hovered, the hairline before an inactive tab that follows another, then
// the badge, the name and the ×, shown on hover and on the active tab.
func (t *topbar) tabFace(gtx C, th *ui.Theme, f topbarFrame, i int, st *tabState) D {
	tab := f.tabs[i]
	active := tab.ID == f.active
	hovered := st.click.Hovered() || st.close.Hovered()
	bg := th.Surface
	size := gtx.Constraints.Min
	r := gtx.Dp(tabRadius)
	shape := clip.RRect{Rect: image.Rectangle{Max: size}, NW: r, NE: r}
	switch {
	case t.sort.Dragging() == tab.ID:
		bg = th.Accent
	case active:
		if s, ok := t.mergeSpan(gtx, f); ok {
			bg = s.color
		} else {
			// A gutter between it and the islands, as between two islands.
			bg, shape.SW, shape.SE = th.Over(ui.Alpha(th.Foreground, tabDetachedFill)), r, r
			shape.Rect.Max.Y -= gtx.Dp(gutter)
		}
	case hovered:
		bg = th.Over(ui.Alpha(th.Foreground, tabHoverFill))
	}
	paint.FillShape(gtx.Ops, bg, shape.Op(gtx.Ops))
	if i > 0 && !active && f.tabs[i-1].ID != f.active {
		inset := gtx.Dp(tabSeparatorIn)
		paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Min: image.Pt(0, inset), Max: image.Pt(gtx.Dp(1), size.Y-inset)}.Op())
	}
	on := th.On(bg)
	fg, weight := on.MutedForeground, font.Normal
	if active {
		fg, weight = on.Foreground, font.Medium
	}
	// Every label sits on the line a detached tab centres it on, so changing
	// tabs never moves one.
	gtx.Constraints = layout.Exact(image.Pt(size.X, size.Y-gtx.Dp(gutter)))
	layout.Inset{Left: tabPadLeft, Right: tabPadRight}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D { return t.tabContent(gtx, on, tab, st, fg, weight, active || hovered) })
	})
	return D{Size: size}
}

// tabContent is the badge, the name and the × in a row.
func (t *topbar) tabContent(gtx C, on *ui.Theme, tab projectTab, st *tabState, fg color.NRGBA, weight font.Weight, showClose bool) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return t.tabBadge(gtx, on, tab.status) }),
		layout.Flexed(1, on.Text(tabTextSize, tab.Name).Leading(tabTextLeading).In(fg).Weight(weight).Layout),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			if !showClose {
				return D{Size: image.Pt(gtx.Dp(16), gtx.Dp(16))}
			}
			return cornerButton(gtx, on, &st.close, icTabClose, false)
		}),
	)
}

// tabBadge is ProjectTab's status glyph and the gap after it, nothing for a
// tab with nothing to say.
func (t *topbar) tabBadge(gtx C, th *ui.Theme, status string) D {
	var draw func(gtx C) D
	switch status {
	case statusBusy, statusCompacting:
		draw = func(gtx C) D { return t.spinner(gtx, th) }
	case statusWaiting:
		draw = func(gtx C) D { return icTabWaiting.Layout(gtx, tabGlyph, th.ToneWait) }
	case statusDone:
		draw = func(gtx C) D { return icTabDone.Layout(gtx, tabGlyph, th.TonePass) }
	default:
		return D{}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(draw), ui.Gap(2))
}

// spinner is LoaderCircle under animate-spin.
func (t *topbar) spinner(gtx C, th *ui.Theme) D {
	turn := float64(time.Since(t.started)%tabSpinPeriod) / float64(tabSpinPeriod)
	half := float32(gtx.Dp(tabGlyph)) / 2
	defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), float32(turn*2*math.Pi))).Push(gtx.Ops).Pop()
	gtx.Execute(op.InvalidateCmd{})
	return icTabLoader.Layout(gtx, tabGlyph, th.MutedForeground)
}

// tabTooltip is ProjectTab's card tooltip: the name, and the path in mono.
func tabTooltip(th *ui.Theme, tab projectTab) layout.Widget {
	return func(gtx C) D {
		path := th.Text(ui.TextXS, tab.Path).Muted().Mono()
		path.MaxLines = 0
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(th.Text(ui.TextXS, tab.Name).Weight(font.Medium).Layout),
			ui.VGap(1.5),
			layout.Rigid(path.Layout),
		)
	}
}

// fillets draw the concave curves that turn the active tab's bottom corners
// into the island's top edge (the mockup's radial-gradient corners). A tab
// flush with the island's left edge has no left fillet: the island's corner
// is square there instead (islandSquare). They stay at the tab's slot, so
// they leave while the tab itself is in the air.
func (t *topbar) fillets(gtx C, f topbarFrame, w int) {
	s, ok := t.mergeSpan(gtx, f)
	if !ok || t.sort.Dragging() == f.active {
		return
	}
	r := gtx.Dp(tabRadius)
	left := t.activeIndex(f)*w - t.list.Position.Offset
	bottom := gtx.Constraints.Max.Y
	if left != s.x0 {
		fillet(gtx, image.Pt(left-r, bottom-r), r, false, s.color)
	}
	fillet(gtx, image.Pt(left+w, bottom-r), r, true, s.color)
}

// mergeSpan is the island the active tab grows out of: the one its whole
// width and both fillets sit over. A tab over the gap between two islands
// grows out of neither. With a single island the tab always merges.
func (t *topbar) mergeSpan(gtx C, f topbarFrame) (islandSpan, bool) {
	at := t.activeIndex(f)
	if at < 0 {
		return islandSpan{}, false
	}
	if len(f.islands) == 0 {
		return islandSpan{x1: math.MaxInt, color: f.island}, true
	}
	r := gtx.Dp(tabRadius)
	left := at*t.tabW - t.list.Position.Offset
	right := left + t.tabW
	for _, s := range f.islands {
		from := left - r
		if left == s.x0 {
			from = left
		}
		if from >= s.x0 && right+r <= s.x1 {
			return s, true
		}
	}
	return islandSpan{}, false
}

func (t *topbar) activeIndex(f topbarFrame) int {
	for i, tab := range f.tabs {
		if tab.ID == f.active {
			return i
		}
	}
	return -1
}

// islandSquare reports the left edge of the island whose top-left corner the
// active tab sits on, which then drops its rounding for the tab to grow out
// of.
func (t *topbar) islandSquare(gtx C, f topbarFrame) (x0 int, ok bool) {
	s, merged := t.mergeSpan(gtx, f)
	if !merged || t.activeIndex(f)*t.tabW-t.list.Position.Offset != s.x0 {
		return 0, false
	}
	return s.x0, true
}

// fillet fills the r×r box at p minus the quarter disc around its top corner
// on the side away from the tab: right is the fillet right of the tab.
func fillet(gtx C, p image.Point, r int, right bool, c color.NRGBA) {
	defer op.Offset(p).Push(gtx.Ops).Pop()
	s := float32(r)
	var path clip.Path
	path.Begin(gtx.Ops)
	if right {
		path.MoveTo(f32.Pt(0, 0))
		path.CubeTo(f32.Pt(0, s*kappa), f32.Pt(s-s*kappa, s), f32.Pt(s, s))
		path.LineTo(f32.Pt(0, s))
	} else {
		path.MoveTo(f32.Pt(s, 0))
		path.CubeTo(f32.Pt(s, s*kappa), f32.Pt(s*kappa, s), f32.Pt(0, s))
		path.LineTo(f32.Pt(s, s))
	}
	path.Close()
	paint.FillShape(gtx.Ops, c, clip.Outline{Path: path.End()}.Op())
}
