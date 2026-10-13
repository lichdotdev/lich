package main

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.
const (
	newSessionHint     = "New session"
	newSessionButton   = "New Session"
	filterSessionsHint = "Filter sessions"
	collapseHint       = "Collapse sidebar"
	allPhasesChip      = "All"
)

var phaseChipLabels = map[sessionPhase]string{
	phaseWaiting: "Waiting", phaseRunning: "Running", phaseUnread: "Unread", phaseIdle: "Idle",
}

var (
	icPlus     = icons.Lucide("plus")
	icSearch   = icons.Lucide("search")
	icCollapse = icons.Lucide("panel-left-close")
)

// sidebarHeader is SessionSidebar's top row and the filter it reveals. The
// filter is never persisted (sidebarFilter says why).
type sidebarHeader struct {
	newMenu    ui.MenuState
	newHover   widget.Clickable // the trigger's hover; the menu takes the press
	search     widget.Clickable
	collapse   widget.Clickable
	open       bool
	query      widget.Editor
	clearQuery widget.Clickable
	// focusQuery asks for the field's focus on the frame after it opens: the
	// press that opened it is the request for it (autoFocus).
	focusQuery    bool
	searchFocused bool
	chips         [5]widget.Clickable // All, then sessionPhases
	phases        map[sessionPhase]bool
}

func (h *sidebarHeader) filter() sidebarFilter {
	if !h.open {
		return sidebarFilter{}
	}
	return sidebarFilter{query: h.query.Text(), phases: h.phases}
}

// toggle opens or closes the filter, clearing it either way: a filter
// outliving its own field would hide cards with nothing on screen to say why.
func (h *sidebarHeader) toggle() {
	h.query.SetText("")
	clear(h.phases)
	h.open = !h.open
	h.focusQuery = h.open
}

// escape is the field's Escape: with text it clears the query and keeps the
// field; on an empty field it closes it, which hands the keyboard back to the
// terminal.
func (h *sidebarHeader) escape() {
	if h.query.Text() != "" {
		h.query.SetText("")
		return
	}
	clear(h.phases)
	h.open = false
}

// headerRow is `mb-2 flex items-center gap-1`: New Session, the filter
// toggle and the collapse button.
func (s *sidebar) headerRow(gtx C, th *ui.Theme, m *model, act *sidebarAction) D {
	h := &s.header
	if h.search.Clicked(gtx) {
		h.toggle()
	}
	if h.collapse.Clicked(gtx) {
		s.setOpen(gtx, false, act)
	}
	entries := append(launchEntries(s.launchProviders, terminalLabel, func(kind string) { act.launch = launchRequest{kind: kind} }),
		ui.MenuEntry{Label: worktreeLabel, Icon: icWtBranch, OnClick: func() { act.worktree = true }})
	trigger := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		tip := th.Tooltip(s.root, s.hint("new"), newSessionHint)
		tip.Side = ui.SideBottom
		return tip.Layout(gtx, sideButton{click: &h.newHover, icon: icPlus, label: newSessionButton, on: h.newMenu.Opened(), morph: &s.morph, glyph: glyphNew}.layout(th))
	}
	square := func(key, label string, b sideButton) layout.FlexChild {
		return layout.Rigid(func(gtx C) D {
			tip := th.Tooltip(s.root, s.hint(key), label)
			tip.Side = ui.SideBottom
			return tip.Layout(gtx, b.layout(th))
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return th.DropdownMenu(s.root, &h.newMenu, entries).Layout(gtx, trigger) }),
		ui.Gap(1),
		square("filter", filterSessionsHint, sideButton{click: &h.search, icon: icSearch, on: h.open}),
		ui.Gap(1),
		square("collapse", collapseHint, sideButton{click: &h.collapse, icon: icCollapse, morph: &s.morph, glyph: glyphToggle}),
	)
}

// filterRow is `mb-2 flex flex-col gap-1.5`: the SearchInput, then the
// phase chips.
func (s *sidebar) filterRow(gtx C, th *ui.Theme, view sidebarView) D {
	h := &s.header
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &h.query, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
			h.escape()
		}
	}
	if h.focusQuery {
		gtx.Execute(key.FocusCmd{Tag: &h.query})
		h.focusQuery = false
	}
	h.searchFocused = gtx.Focused(&h.query)
	field := th.Input(&h.query, filterSessionsHint)
	field.Leading, field.Height, field.TextSize, field.Clear = icSearch, ui.Space(7), ui.TextXS, &h.clearQuery
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(field.Layout),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return s.phaseChips(gtx, th, view.counts) }),
	)
}

// phaseChips is SessionPhaseChips: All is on while no phase is, and clears
// them; every other chip toggles its phase.
func (s *sidebar) phaseChips(gtx C, th *ui.Theme, counts map[sessionPhase]int) D {
	h := &s.header
	if h.chips[0].Clicked(gtx) {
		clear(h.phases)
	}
	total := 0
	for i, p := range sessionPhases {
		total += counts[p]
		if !h.chips[i+1].Clicked(gtx) {
			continue
		}
		if h.phases[p] {
			delete(h.phases, p)
		} else {
			h.phases[p] = true
		}
	}
	kids := []ui.WrapChild{ui.WrapItem(th.Chip(&h.chips[0], allPhasesChip, total, len(h.phases) == 0).Layout)}
	for i, p := range sessionPhases {
		kids = append(kids, ui.WrapItem(th.Chip(&h.chips[i+1], phaseChipLabels[p], counts[p], h.phases[p]).Layout))
	}
	return ui.Wrap{GapX: ui.Space(0.5), GapY: ui.Space(0.5)}.Layout(gtx, kids...)
}

// sideButton is the sidebar's ghost buttons. Their className's
// hover:bg-accent replaces the variant's hover:bg-muted, but tailwind-merge
// keeps the variant's dark:hover:bg-muted/50, which wins in the dark theme:
// so the header hovers like the kit's ghost, while on (aria-pressed or
// aria-expanded) is bg-accent. The rail hovers bg-accent/60. A labelled
// one is SessionSidebar's New Session, `h-9 justify-start gap-2 px-2.5`, its
// text foreground and its icon muted; a square one is `size-8`, muted until
// hovered or on.
type sideButton struct {
	click *widget.Clickable
	icon  *icons.Icon
	label string
	on    bool    // aria-pressed or aria-expanded: bg-accent
	hover float32 // the hover fill's bg-accent opacity, 0 for the ghost's
	// glyph names the icon for the sidebar's morph, "" for none.
	morph *sidebarMorph
	glyph string
}

// ghostHoverFill is button.tsx ghost's dark:hover:bg-muted/50.
const ghostHoverFill = 0.5

func (b sideButton) layout(th *ui.Theme) layout.Widget {
	return func(gtx C) D {
		return b.click.Layout(gtx, func(gtx C) D {
			hovered := b.click.Hovered()
			var bg color.NRGBA
			switch {
			case b.on:
				bg = th.Accent
			case hovered && b.hover > 0:
				bg = th.Over(ui.Alpha(th.Accent, b.hover))
			case hovered:
				bg = th.Over(ui.Alpha(th.Muted, ghostHoverFill))
			}
			fg := th.MutedForeground
			if b.on || hovered {
				fg = th.Foreground
			}
			if b.label == "" {
				return squareFace(gtx, bg, b.iconWidget(fg))
			}
			face := th.On(opaqueOr(bg, th))
			return labelFace(gtx, face, bg, b.iconWidget(face.MutedForeground), b.label)
		})
	}
}

func (b sideButton) iconWidget(fg color.NRGBA) layout.Widget {
	icon := func(gtx C) D { return b.icon.Layout(gtx, ui.Space(4), fg) }
	if b.glyph == "" {
		return icon
	}
	return func(gtx C) D { return b.morph.glyph(gtx, b.glyph, icon) }
}

// opaqueOr is bg, or the surface under it while bg paints nothing.
func opaqueOr(bg color.NRGBA, th *ui.Theme) color.NRGBA {
	if bg.A == 0 {
		return th.Surface
	}
	return bg
}

func squareFace(gtx C, bg color.NRGBA, icon layout.Widget) D {
	size := gtx.Dp(ui.Space(8))
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
		func(gtx C) D { return layout.Center.Layout(gtx, icon) },
	)
}

func labelFace(gtx C, th *ui.Theme, bg color.NRGBA, icon layout.Widget, label string) D {
	h := gtx.Dp(ui.Space(9))
	gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = h, h
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.Y = h
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(icon),
					ui.Gap(2),
					layout.Flexed(1, th.Text(ui.TextSM, label).Weight(font.Medium).Layout),
				)
			})
		},
	)
}

// noticeText is common/Notice at the sidebar's `px-2 py-3`.
func noticeText(gtx C, th *ui.Theme, text string) D {
	l := th.Text(ui.TextXS, text).Muted()
	l.MaxLines = 0
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(3), Bottom: ui.Space(3)}.Layout(gtx, l.Layout)
}
