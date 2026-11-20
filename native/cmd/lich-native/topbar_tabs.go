package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The project tab, from the approved mockup: browser-style tabs of a fixed
// width that shrinks evenly to a floor before the strip scrolls.
var (
	tabWidthMax  = ui.Space(49)   // 196px
	tabWidthMin  = ui.Space(29.5) // 118px
	tabHeight    = ui.Space(9.5)  // 38px, from 6px under the band's top
	tabTop       = ui.Space(1.5)
	tabRadius    = ui.RadiusLG
	tabPadLeft   = ui.Space(3.5)
	tabPadRight  = ui.Space(2.5)
	tabGlyph     = ui.Space(3.5)
	tabSeparator = ui.Space(4) // the hairline between the pinned tabs and the rest, centred on the pill
)

const (
	tabTextSize    = 13
	tabTextLeading = 20
	// tabHoverFill is the mockup's --hover-faint, the foreground at 4%.
	tabHoverFill = 0.04
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
	w := tabWidth(gtx.Constraints.Max.X-plus, len(f.tabs), gtx.Dp(tabWidthMax), gtx.Dp(tabWidthMin))
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

// tabFace draws a tab: a pill of Accent when active or carried, a faint
// fill when hovered, the hairline where the pinned tabs end, then the badge,
// the name and the ×, shown on hover and on the active tab.
func (t *topbar) tabFace(gtx C, th *ui.Theme, f topbarFrame, i int, st *tabState, act *topbarAction) D {
	tab := f.tabs[i]
	active := tab.ID == f.active
	hovered := st.click.Hovered() || st.close.Hovered()
	size := gtx.Constraints.Min
	// The pill stands a gutter clear of the islands under it, as they are
	// of each other.
	pill := image.Pt(size.X, size.Y-gtx.Dp(gutter))
	bg := th.Surface
	switch {
	case active || t.sort.Dragging() == tab.ID:
		bg = th.Accent
	case hovered:
		bg = th.Over(ui.Alpha(th.Foreground, tabHoverFill))
	}
	if bg != th.Surface {
		paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: pill}, gtx.Dp(tabRadius)).Op(gtx.Ops))
	}
	if i > 0 && f.tabs[i-1].pinned != tab.pinned {
		top := (pill.Y - gtx.Dp(tabSeparator)) / 2
		paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Min: image.Pt(0, top), Max: image.Pt(gtx.Dp(1), top+gtx.Dp(tabSeparator))}.Op())
	}
	on := th.On(bg)
	fg, weight := on.MutedForeground, font.Normal
	if active {
		fg, weight = on.Foreground, font.Medium
	}
	gtx.Constraints = layout.Exact(pill)
	layout.Inset{Left: tabPadLeft, Right: tabPadRight}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D { return t.tabContent(gtx, on, tab, st, fg, weight, active || hovered, act) })
	})
	return D{Size: size}
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
		layout.Rigid(func(gtx C) D {
			if !tab.pinned {
				return D{}
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(func(gtx C) D { return icTabPin.Layout(gtx, tabGlyph, fg) }), ui.Gap(2))
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
			if !showClose || tab.pinned {
				return D{Size: image.Pt(gtx.Dp(16), gtx.Dp(16))}
			}
			return on.CornerButton(&st.close, icTabClose, false).Layout(gtx)
		}),
	)
}

// tabBadge is ProjectTab's status glyph and the gap after it, nothing for a
// tab with nothing to say.
func (t *topbar) tabBadge(gtx C, th *ui.Theme, status string) D {
	var draw func(gtx C) D
	switch status {
	case statusBusy, statusCompacting:
		draw = func(gtx C) D { return th.Spinner(gtx, tabGlyph, th.MutedForeground) }
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
