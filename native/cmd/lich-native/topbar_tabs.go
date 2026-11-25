package main

import (
	"image"
	"image/color"
	"sort"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The project tab, from the approved mockup: a pill as wide as its name, up
// to a ceiling; when they do not all fit the widest give way evenly to a floor
// before the strip scrolls.
var (
	tabWidthMax = ui.Space(41)   // 164px
	tabWidthMin = ui.Space(29.5) // 118px
	tabHeight   = ui.Space(8)    // 32px, the pill, from 6px under the band's top
	tabTop      = ui.Space(1.5)
	tabRadius   = ui.RadiusLG
	tabPadLeft  = ui.Space(3.5)
	tabPadRight = ui.Space(2.5)
	tabGlyph    = ui.Space(3.5)
	// tabSlot is the box at a tab's end that holds the × or the status badge.
	tabSlot           = unit.Dp(16)
	tabSeparatorWidth = unit.Dp(1)
	tabGap            = unit.Dp(5)
	tabGroupMargin    = unit.Dp(5)
)

const (
	// tabNameGap, in Space steps, keeps the name off the slot at its end.
	tabNameGap     = 3
	tabTextSize    = 13
	tabTextLeading = 20
	// tabHoverFill is the mockup's --hover-faint, the foreground at 4%.
	tabHoverFill = 0.04
	// tabRestMix lifts a resting tab's name from MutedForeground toward
	// Foreground, which alone read too faint on the canvas.
	tabRestMix = 0.4
)

var (
	icTabClose   = icons.Lucide("x")
	icTabWaiting = icons.Lucide("bell")
	icTabDone    = icons.Lucide("check")
	icTabLayout  = icons.Lucide("layout-panel-left")
	icTabPin     = icons.Lucide("pin")
)

type tabState struct {
	click ui.Clickable
	close ui.Clickable
	tip   ui.Tooltip
	menu  ui.MenuState
	// rename is a layout tab's name as a field, while it is renamed.
	rename ui.InlineEdit
	// dots is the tab's status while it has one.
	dots *tabDots
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

// tabWidths is each tab's width: its natural one, except that when they
// together overflow room the widest are cut to a shared cap, never under
// floor, past which the strip scrolls.
func tabWidths(room int, natural []int, floor int) []int {
	order := make([]int, len(natural))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return natural[order[a]] < natural[order[b]] })
	out := make([]int, len(natural))
	left := room
	for k, i := range order {
		share := max(floor, left/(len(order)-k))
		out[i] = min(natural[i], share)
		left -= out[i]
	}
	return out
}

func tabMargins(tabs []projectTab, i int) (left, right unit.Dp) {
	if i > 0 && tabs[i-1].pinned != tabs[i].pinned {
		left = tabGroupMargin + tabSeparatorWidth
	}
	if i+1 < len(tabs) {
		right = tabGap
	}
	return left, right
}

func tabMarginPixels(gtx C, tabs []projectTab, i int) (left, right int) {
	l, r := tabMargins(tabs, i)
	return gtx.Dp(l), gtx.Dp(r)
}

// tabNatural is the width a tab wants: its padding, glyphs, name and the slot
// at its end, capped at tabWidthMax. The name is measured in the active
// weight so a tab does not grow when it is picked.
func (t *topbar) tabNatural(gtx C, th *ui.Theme, tab projectTab) int {
	m := op.Record(gtx.Ops)
	gtx.Constraints = layout.Constraints{Max: image.Pt(1<<24, gtx.Constraints.Max.Y)}
	name := th.Text(tabTextSize, tab.Name).Leading(tabTextLeading).Weight(font.Medium).Layout(gtx).Size.X
	m.Stop()
	w := gtx.Dp(tabPadLeft) + name + gtx.Dp(ui.Space(tabNameGap)) + gtx.Dp(tabSlot) + gtx.Dp(tabPadRight)
	if tab.layout {
		w += gtx.Dp(tabGlyph) + gtx.Dp(ui.Space(2))
	}
	if tab.pinned {
		w += gtx.Dp(tabGlyph) + gtx.Dp(ui.Space(2))
	}
	return min(w, gtx.Dp(tabWidthMax))
}

// strip is the scrolling row of tabs with the open-project button right after
// the last one.
func (t *topbar) strip(gtx C, th *ui.Theme, f topbarFrame, act *topbarAction) D {
	// A tab being renamed takes presses for its field, not for a drag.
	t.sort.Disabled = t.holdsKeys()
	before := t.sort.Dragging()
	if d, ok := t.sort.Update(gtx); ok && d.ID != d.Over {
		act.drop = d
	}
	dragEnded := before != "" && t.sort.Dragging() == ""
	plus := gtx.Dp(ui.BarButtonSize) + gtx.Dp(barGap)
	totalMargins := 0
	natural := make([]int, len(f.tabs))
	for i, tab := range f.tabs {
		natural[i] = t.tabNatural(gtx, th, tab)
		left, right := tabMarginPixels(gtx, f.tabs, i)
		totalMargins += left + right
	}
	w := tabWidths(gtx.Constraints.Max.X-plus-totalMargins, natural, gtx.Dp(tabWidthMin))
	t.lay(gtx, f, w)
	ids := make([]string, len(f.tabs))
	for i, tab := range f.tabs {
		ids[i] = tab.ID
	}
	size := gtx.Constraints.Max
	clipped := clip.Rect{Max: size}.Push(gtx.Ops)
	// Sized to its tabs, the list leaves the strip's empty run to the band
	// under it, which drags the window.
	content := gtx
	content.Constraints.Min.X = 0
	t.list.Layout(content, 1, func(gtx C, _ int) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				// The pills stand a gutter clear of the islands under them;
				// tabs have a 5px gap and the group break has its own margins.
				d := layout.Inset{Top: tabTop, Bottom: gutter}.Layout(gtx, func(gtx C) D {
					return t.sort.List(gtx, ui.SortList{Horizontal: true, Within: true}, ids, func(gtx C, i int) D {
						return t.tab(gtx, th, f, i, w, dragEnded, act)
					})
				})
				return d
			}),
			ui.Gap(1),
			// Centred on the band, which puts the "+" on the tabs' label line.
			layout.Rigid(func(gtx C) D { return t.openButton(gtx, th, act) }),
		)
	})
	clipped.Pop()
	return D{Size: size}
}

// laidTab is a strip tab and where it was drawn.
type laidTab struct {
	id   string
	rect image.Rectangle
}

// lay records where each tab of the strip, widths[i] wide and scrolled as the
// list is, sits in the window, cut to the strip's own width.
func (t *topbar) lay(gtx C, f topbarFrame, widths []int) {
	t.laid = t.laid[:0]
	top := gtx.Dp(tabTop)
	strip := image.Rect(f.stripX, 0, f.stripX+gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	x := f.stripX - t.list.Position.Offset
	for i, tab := range f.tabs {
		left, right := tabMarginPixels(gtx, f.tabs, i)
		x += left
		r := image.Rect(x, top, x+widths[i], top+gtx.Dp(tabHeight)).Intersect(strip)
		t.laid = append(t.laid, laidTab{id: tab.ID, rect: r})
		x += widths[i] + right
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

func (t *topbar) tab(gtx C, th *ui.Theme, f topbarFrame, i int, widths []int, dragEnded bool, act *topbarAction) D {
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
	left, right := tabMargins(f.tabs, i)
	leftPx, rightPx := tabMarginPixels(gtx, f.tabs, i)
	size := image.Pt(widths[i], gtx.Dp(tabHeight))
	gtx.Constraints = layout.Exact(image.Pt(size.X+leftPx+rightPx, size.Y))
	entries := []ui.MenuEntry{
		{Label: i18n.T("tabs.topbarTabs.pinProject"), Icon: icTabPin, OnClick: func() { act.pinProject, act.pin = tab.ID, !tab.pinned }},
		{Kind: ui.MenuSeparator},
		{Label: i18n.T("tabs.topbarTabs.projectSettings"), Icon: icSettings, OnClick: func() { act.projectSettings = tab.ID }},
		{Kind: ui.MenuSeparator},
		{Label: i18n.T("tabs.topbarTabs.closeProject"), Icon: icMenuClose, Variant: ui.VariantDestructive, OnClick: func() { act.ask = tab.ID }},
	}
	if tab.pinned {
		entries[0].Label = i18n.T("tabs.topbarTabs.unpinProject")
	}
	if tab.layout {
		entries = []ui.MenuEntry{
			{Label: i18n.T("layouts.tab.rename"), Icon: icMenuRename, OnClick: func() { st.rename.Start(tab.Name) }},
			{Kind: ui.MenuSeparator},
			{Label: i18n.T("layouts.tab.close"), Icon: icMenuClose, OnClick: func() { act.ask = tab.ID }},
		}
	}
	return layout.Inset{Left: left, Right: right}.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(size)
		return t.sort.Item(gtx, tab.ID, func(gtx C) D {
			return th.ContextMenu(t.root, &st.menu, entries).Layout(gtx, func(gtx C) D {
				tip := th.TooltipCard(t.root, &st.tip, tabTooltip(th.On(th.Popover), tab))
				tip.Side = ui.SideBottom
				return tip.Layout(gtx, func(gtx C) D {
					return st.click.Layout(gtx, func(gtx C) D { return t.tabFace(gtx, th, f, i, st, act) })
				})
			})
		}, nil)
	})
}

// tabFace draws a tab: the Sidebar surface and a Border hairline when active,
// Accent on a carried tab, a faint fill when hovered and, on a carried tab, the
// low elevation.
func (t *topbar) tabFace(gtx C, th *ui.Theme, f topbarFrame, i int, st *tabState, act *topbarAction) D {
	tab := f.tabs[i]
	active := tab.ID == f.active
	dragging := t.sort.Dragging() == tab.ID
	hovered := st.click.Hovered() || st.close.Hovered()
	pill := gtx.Constraints.Min
	bg := th.Surface
	switch {
	case dragging:
		bg = th.Accent
	case active:
		bg = th.Sidebar
	case hovered:
		bg = th.Over(ui.Alpha(th.Foreground, tabHoverFill))
	}
	if dragging {
		ui.ElevationLow.Paint(gtx, pill, gtx.Dp(tabRadius))
	}
	if bg != th.Surface {
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: pill}, gtx.Dp(tabRadius)).Op(gtx.Ops))
	}
	on := th.On(bg)
	if active || hovered || dragging {
		ui.InsetBorder(gtx, pill, gtx.Dp(tabRadius), on.Over(on.Border))
	}
	fg, weight := ui.MixOKLab(on.MutedForeground, on.Foreground, tabRestMix), font.Normal
	if active {
		fg, weight = on.Foreground, font.Medium
	}
	showClose := t.sort.Dragging() == "" && (active || hovered)
	gtx.Constraints = layout.Exact(pill)
	layout.Inset{Left: tabPadLeft, Right: tabPadRight}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D { return t.tabContent(gtx, on, tab, st, fg, weight, showClose, act) })
	})
	return D{Size: pill}
}

// tabContent is the name and, in the slot at its end, the × or the badge.
func (t *topbar) tabContent(gtx C, on *ui.Theme, tab projectTab, st *tabState, fg color.NRGBA, weight font.Weight, showClose bool, act *topbarAction) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if !tab.layout {
				return D{}
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(func(gtx C) D { return icTabLayout.Layout(gtx, tabGlyph, fg) }), ui.Gap(2))
		}),
		layout.Rigid(func(gtx C) D {
			if !tab.pinned {
				return D{}
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(func(gtx C) D { return icTabPin.Layout(gtx, tabGlyph, fg) }), ui.Gap(2))
		}),
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
		ui.Gap(tabNameGap),
		layout.Rigid(func(gtx C) D {
			if !showClose || tab.pinned {
				slot := image.Pt(gtx.Dp(tabSlot), gtx.Dp(tabSlot))
				gtx.Constraints = layout.Exact(slot)
				return layout.Center.Layout(gtx, func(gtx C) D { return t.tabBadge(gtx, on, st, tab.status) })
			}
			return on.CornerButton(&st.close, icTabClose, false).Layout(gtx)
		}),
	)
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
