package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// SidebarRail's buttons hover bg-accent/60.
const railHoverFill = 0.6

var icExpand = icons.Lucide("panel-left")

// sidebarRail is SidebarRail: the collapsed sidebar, the same list in the
// same order reduced to each card's status ring. It selects and it creates,
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
// session of the default provider, then every block's cards, a hairline
// between blocks where the open sidebar draws a titled divider. The pb-1.5
// is the window's gutter around the sidebar, as the open one's mb-1.5 is.
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

// railItem is one row of the rail's list: a card, or the hairline before a
// block.
type railItem struct {
	session *sessionView
}

func (r *sidebarRail) sessions(gtx C, th *ui.Theme, s *sidebar, blocks []blockView, active string, act *sidebarAction) D {
	var items []railItem
	for bi := range blocks {
		if bi > 0 {
			items = append(items, railItem{})
		}
		for si := range blocks[bi].sessions {
			items = append(items, railItem{session: &blocks[bi].sessions[si]})
		}
	}
	// The thumb floats over the list: a reserved gutter would leave 32px cells
	// a narrower column, cutting them off on the right.
	list := th.ScrollArea(&r.list)
	list.AnchorStrategy = material.Overlay
	return list.Layout(gtx, len(items), func(gtx C, i int) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.N.Layout(gtx, func(gtx C) D {
			if items[i].session == nil {
				return railHairline(gtx, th)
			}
			top := ui.Space(1)
			if i == 0 {
				top = 0
			}
			return layout.Inset{Top: top}.Layout(gtx, func(gtx C) D { return r.session(gtx, th, s, items[i].session, active, act) })
		})
	})
}

// railHairline is `my-1 h-px w-5 bg-border`, inside the list's gap-1.
func railHairline(gtx C, th *ui.Theme) D {
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(ui.Space(5)), gtx.Dp(1)))
		return ui.Fill(gtx, th.Over(th.Border), 0)
	})
}

// session is RailSession: `size-8 rounded-md` around the card's own status
// ring, lit while active, under the card's own tooltip.
func (r *sidebarRail) session(gtx C, th *ui.Theme, s *sidebar, sv *sessionView, active string, act *sidebarAction) D {
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
	return tooltip.Layout(gtx, func(gtx C) D {
		return click.Layout(gtx, func(gtx C) D {
			bg := th.Surface
			switch {
			case sv.ID == active:
				bg = th.Accent
			case click.Hovered():
				bg = th.Over(ui.Alpha(th.Accent, railHoverFill))
			}
			return squareFace(gtx, bg, func(gtx C) D { return s.statusRing(gtx, th.On(bg), sv) })
		})
	})
}
