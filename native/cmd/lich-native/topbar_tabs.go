package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The project tab, from the approved mockup: browser-style tabs of a fixed
// width that shrinks evenly to a floor before the strip scrolls.
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
	// kappa places a cubic's controls so it draws a quarter circle.
	kappa = 0.5523
)

var (
	icTabClose   = icons.Lucide("x")
	icTabWaiting = icons.Lucide("bell")
	icTabDone    = icons.Lucide("check")
	icTabLayout  = icons.Lucide("layout-panel-left")
)

type tabState struct {
	click widget.Clickable
	close widget.Clickable
	tip   ui.Tooltip
	menu  ui.MenuState
	// rename is a layout tab's name as a field, while it is renamed.
	rename ui.InlineEdit
}

// holdsKeys reports whether a tab's name is being edited, which keeps the
// keyboard off the terminal.
func (t *topbar) holdsKeys() bool {
	for _, st := range t.tabs {
		if st.rename.Active() {
			return true
		}
	}
	return false
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
	// A tab being renamed takes presses for its field, not for a drag.
	t.sort.Disabled = t.holdsKeys()
	before := t.sort.Dragging()
	if d, ok := t.sort.Update(gtx); ok && d.ID != d.Over {
		act.drop = d
	}
	dragEnded := before != "" && t.sort.Dragging() == ""
	plus := gtx.Dp(barButton) + gtx.Dp(barGap)
	w := tabWidth(gtx.Constraints.Max.X-plus, len(f.tabs), gtx.Dp(tabWidthMax), gtx.Dp(tabWidthMin))
	t.tabW = w
	t.lay(gtx, f, w)
	ids := make([]string, len(f.tabs))
	for i, tab := range f.tabs {
		ids[i] = tab.ID
	}
	size := gtx.Constraints.Max
	// A strip pushed past the island's edge reaches left over the room
	// stripStart left, where the active first tab's left fillet draws.
	var filletRoom int
	if t.shift > 0 {
		filletRoom = gtx.Dp(tabRadius)
	}
	clipped := clip.Rect{Min: image.Pt(-filletRoom, 0), Max: size}.Push(gtx.Ops)
	tabs := t.list.Layout(gtx, 1, func(gtx C, _ int) D {
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
	if f.chrome.drawn() {
		// The strip's empty run drags the window, and a double click on it
		// maximizes, as a title bar does.
		area := clip.Rect{Min: image.Pt(tabs.Size.X, 0), Max: size}.Push(gtx.Ops)
		system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
		area.Pop()
	}
	clipped.Pop()
	return D{Size: size}
}

// laidTab is a strip tab and where it was drawn.
type laidTab struct {
	id   string
	rect image.Rectangle
}

// lay records where each tab of the strip, w wide and scrolled as the list
// is, sits in the window, cut to the strip's own width.
func (t *topbar) lay(gtx C, f topbarFrame, w int) {
	t.laid = t.laid[:0]
	top := gtx.Dp(tabTop)
	strip := image.Rect(f.stripX, 0, f.stripX+gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	for i, tab := range f.tabs {
		x := f.stripX + i*w - t.list.Position.Offset
		r := image.Rect(x, top, x+w, top+gtx.Dp(tabHeight)).Intersect(strip)
		t.laid = append(t.laid, laidTab{id: tab.ID, rect: r})
	}
}

// tabAt is the strip tab at window point p, "" for none.
func (t *topbar) tabAt(p image.Point) string {
	for _, l := range t.laid {
		if p.In(l.rect) {
			return l.id
		}
	}
	return ""
}

func (t *topbar) tab(gtx C, th *ui.Theme, f topbarFrame, i, w int, dragEnded bool, act *topbarAction) D {
	tab := f.tabs[i]
	st := t.tabState(tab.ID)
	closed, clicked := st.close.Clicked(gtx), false
	for {
		c, ok := st.click.Update(gtx)
		if !ok {
			break
		}
		clicked = true
		if c.NumClicks >= 2 && tab.layout {
			st.rename.Start(tab.Name)
		}
	}
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
		{Label: i18n.T("tabs.topbarTabs.projectSettings"), Icon: icSettings, OnClick: func() { act.projectSettings = tab.ID }},
		{Kind: ui.MenuSeparator},
		{Label: i18n.T("tabs.topbarTabs.closeProject"), Icon: icMenuClose, Variant: ui.VariantDestructive, OnClick: func() { act.ask = tab.ID }},
	}
	if tab.layout {
		entries = []ui.MenuEntry{
			{Label: i18n.T("layouts.tab.rename"), Icon: icMenuRename, OnClick: func() { st.rename.Start(tab.Name) }},
			{Kind: ui.MenuSeparator},
			{Label: i18n.T("layouts.tab.close"), Icon: icMenuClose, OnClick: func() { act.ask = tab.ID }},
		}
	}
	return t.sort.Item(gtx, tab.ID, func(gtx C) D {
		return th.ContextMenu(t.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
			tip := th.TooltipCard(t.root, &st.tip, tabTooltip(th.On(th.Popover), tab))
			tip.Side = ui.SideBottom
			return tip.Layout(gtx, func(gtx C) D {
				return st.click.Layout(gtx, func(gtx C) D { return t.tabFace(gtx, th, f, i, st, act) })
			})
		})
	}, nil)
}

// tabFace draws a tab: the island's color when active, a faint fill when
// hovered, the hairline before an inactive tab that follows another, then
// the badge, the name and the ×, shown on hover and on the active tab.
func (t *topbar) tabFace(gtx C, th *ui.Theme, f topbarFrame, i int, st *tabState, act *topbarAction) D {
	tab := f.tabs[i]
	active := tab.ID == f.active
	hovered := st.click.Hovered() || st.close.Hovered()
	bg := th.Surface
	size := gtx.Constraints.Min
	r := gtx.Dp(tabRadius)
	shape := clip.RRect{Rect: image.Rectangle{Max: size}, NW: r, NE: r}
	switch {
	case t.sort.Dragging() == tab.ID:
		// In the air it grows out of no island.
		bg, shape = th.Accent, detachedTab(gtx, size, r)
	case active:
		s, k := t.mergeShare(gtx, f)
		bg, shape = ui.Over(th.Over(ui.Alpha(th.Foreground, tabDetachedFill)), ui.Alpha(s.color, k)), mergingTab(gtx, size, r, k)
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
		return layout.W.Layout(gtx, func(gtx C) D { return t.tabContent(gtx, on, tab, st, fg, weight, active || hovered, act) })
	})
	return D{Size: size}
}

// mergingTab is a tab k of the way from detached (0) to grown into its
// island (1): its foot comes down the gutter and its bottom corners square.
func mergingTab(gtx C, size image.Point, r int, k float32) clip.RRect {
	shape := detachedTab(gtx, size, r)
	shape.Rect.Max.Y += int(k * float32(gtx.Dp(gutter)))
	shape.SW = int((1 - k) * float32(r))
	shape.SE = shape.SW
	return shape
}

// detachedTab is a tab that grows out of no island: rounded all round, a
// gutter clear of the islands as they are of each other.
func detachedTab(gtx C, size image.Point, r int) clip.RRect {
	return clip.RRect{Rect: image.Rectangle{Max: image.Pt(size.X, size.Y-gtx.Dp(gutter))}, NW: r, NE: r, SE: r, SW: r}
}

// tabContent is the badge, the name and the × in a row.
func (t *topbar) tabContent(gtx C, on *ui.Theme, tab projectTab, st *tabState, fg color.NRGBA, weight font.Weight, showClose bool, act *topbarAction) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if !tab.layout {
				return D{}
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(func(gtx C) D { return icTabLayout.Layout(gtx, tabGlyph, fg) }), ui.Gap(2))
		}),
		layout.Rigid(func(gtx C) D { return t.tabBadge(gtx, on, tab.status) }),
		layout.Flexed(1, func(gtx C) D {
			if !st.rename.Active() {
				return on.Text(tabTextSize, tab.Name).Leading(tabTextLeading).In(fg).Weight(weight).Layout(gtx)
			}
			d, res := on.InlineEdit(&st.rename, tabTextSize).Weight(weight).Layout(gtx)
			if res.Outcome == ui.InlineEditCommitted {
				act.renamed = layoutRename{id: tab.ID, name: res.Value}
			}
			return d
		}),
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
		draw = func(gtx C) D { return ui.Spinner(gtx, tabGlyph, th.MutedForeground) }
	case statusWaiting:
		draw = func(gtx C) D { return icTabWaiting.Layout(gtx, tabGlyph, th.ToneWait) }
	case statusDone:
		draw = func(gtx C) D { return icTabDone.Layout(gtx, tabGlyph, th.TonePass) }
	default:
		return D{}
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(draw), ui.Gap(2))
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
// is square there instead (islandSquare). None is drawn while a tab is in
// the air (mergeSpan).
func (t *topbar) fillets(gtx C, f topbarFrame, w int) {
	s, k := t.mergeShare(gtx, f)
	r := int(k * float32(gtx.Dp(tabRadius)))
	if r == 0 {
		return
	}
	left := t.activeIndex(f)*w - t.list.Position.Offset
	bottom := gtx.Constraints.Max.Y
	if left != s.x0 {
		fillet(gtx, image.Pt(left-r, bottom-r), r, false, s.color)
	}
	fillet(gtx, image.Pt(left+w, bottom-r), r, true, s.color)
}

// mergeShare is how far the active tab has grown into the island it merges
// with, 0 detached to 1 merged, easing between the two on the sidebar's
// curve, and that island (the last one it merged with while it leaves).
// Another tab becoming active snaps rather than animates: only a tab leaving
// or landing in its island eases.
func (t *topbar) mergeShare(gtx C, f topbarFrame) (islandSpan, float32) {
	s, ok := t.mergeSpan(gtx, f)
	if ok {
		t.merged = s
	}
	if f.active != t.mergedFor {
		t.mergedFor = f.active
		t.merge.snap(ok)
	}
	return t.merged, t.merge.at(gtx, ok)
}

// mergeSpan is the island the active tab grows out of: the one its whole
// width and both fillets sit over. A tab over the gap between two islands
// grows out of neither. With a single island the tab always merges.
func (t *topbar) mergeSpan(gtx C, f topbarFrame) (islandSpan, bool) {
	at := t.activeIndex(f)
	// While a tab is in the air the others slide out of its way, so none
	// grows out of an island: the fillets and square corner would stay
	// behind at the active tab's slot.
	if at < 0 || t.sort.Dragging() != "" {
		return islandSpan{}, false
	}
	if len(f.islands) == 0 {
		return islandSpan{x0: -t.shift, x1: math.MaxInt, color: f.island}, true
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
	if x0, ok := t.hoverSquare(f); ok {
		return x0, true
	}
	s, merged := t.mergeSpan(gtx, f)
	// Square only once the tab has fully grown in: the island's corner
	// rounds again as soon as the tab starts to leave. A tab not eased yet
	// is merged outright, as mergeShare snaps it.
	growing := t.mergedFor == f.active && t.merge.shown < 1
	if !merged || growing || t.activeIndex(f)*t.tabW-t.list.Position.Offset != s.x0 {
		return 0, false
	}
	return s.x0, true
}

// hoverSquare is the first island's left edge while the first tab, flush
// with it and not active, is hovered: the hover reaches down to the island,
// and the island's rounded corner under its square foot showed as a curve.
func (t *topbar) hoverSquare(f topbarFrame) (x0 int, ok bool) {
	if len(f.tabs) == 0 || f.tabs[0].ID == f.active || t.sort.Dragging() != "" {
		return 0, false
	}
	st := t.tabs[f.tabs[0].ID]
	if st == nil || !(st.click.Hovered() || st.close.Hovered()) {
		return 0, false
	}
	x0 = -t.shift
	if len(f.islands) > 0 {
		x0 = f.islands[0].x0
	}
	return x0, -t.list.Position.Offset == x0
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
