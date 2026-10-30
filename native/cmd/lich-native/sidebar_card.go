package main

import (
	"image"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/lichdotdev/lich/native/ui"
)

func clickable(m map[string]*widget.Clickable, key string) *widget.Clickable {
	c, ok := m[key]
	if !ok {
		c = new(widget.Clickable)
		m[key] = c
	}
	return c
}

// card is SessionCard: folders are where its menu can file it, every folder
// of the project but its own.
func (s *sidebar) card(gtx C, th *ui.Theme, sv *sessionView, active bool, folders []string) (D, sidebarAction) {
	var act sidebarAction
	click, closeClick := clickable(s.cards, sv.ID), clickable(s.closes, sv.ID)
	if click.Clicked(gtx) {
		act.activate = sv.ID
	}
	if closeClick.Clicked(gtx) {
		act.close = sv.ID
	}
	if clickable(s.mothers, sv.ID).Clicked(gtx) {
		act.fold = motherKey(sv.ID)
	}
	prChip := clickable(s.prChips, sv.ID)
	if prChip.Clicked(gtx) && sv.pr != nil {
		act.activate = sv.ID
		act.openPR, act.openPRURL = prTarget(sv.pr, sv.shown, sv.path)
	}
	fill := cardFill(th, sv.Color, active, click.Hovered())
	th = th.On(fill)
	tip, ok := s.tips[sv.ID]
	if !ok {
		tip = new(ui.Tooltip)
		s.tips[sv.ID] = tip
	}
	menu, ok := s.menus[sv.ID]
	if !ok {
		menu = new(ui.MenuState)
		s.menus[sv.ID] = menu
	}
	tooltip := th.TooltipCard(s.root, tip, sessionTooltip(th, sv))
	tooltip.Side = ui.SideRight
	rename, ok := s.renames[sv.ID]
	if !ok {
		rename = new(ui.InlineEdit)
		s.renames[sv.ID] = rename
	}
	d := th.ContextMenuLazy(s.root, menu, func() []ui.MenuEntry {
		var extra []ui.MenuEntry
		if len(sv.continueIn) > 0 {
			extra = append(extra, continueInItem(th, sv, &act))
		}
		return cardMenu(sv, folders, &act, func() { rename.Start(sv.Label) }, func() { s.askFolderName([]string{sv.ID}) }, extra...)
	}).Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D { return s.cardFace(gtx, th, sv, click, closeClick, prChip, fill) })
	})
	if s.renamed.id == sv.ID {
		act.rename, s.renamed = s.renamed, sessionRename{}
	}
	if act.copyText != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(act.copyText))})
	}
	return d, act
}

// cardFace is the card itself, under its tooltip.
func (s *sidebar) cardFace(gtx C, th *ui.Theme, sv *sessionView, click, closeClick, prChip *widget.Clickable, fill color.NRGBA) D {
	return click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
			func(gtx C) D {
				return withLinks(gtx, th, sv.place.sessionRowLinks, func(gtx C) D { return s.cardBody(gtx, th, sv, prChip, closeClick, click.Hovered()) })
			},
		)
	})
}

// inHand is a session carried over the window, a card out of the sidebar
// or a pane by its header: a pill with its ring and name just right of the
// pointer at p, or left of it where the window ends first.
func (s *sidebar) inHand(gtx C, th *ui.Theme, sv *sessionView, p image.Point) {
	size := gtx.Constraints.Max
	on := th.On(th.Popover)
	m := op.Record(gtx.Ops)
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(cardInHandWidth), size.Y)}
	pill := layout.Inset{Left: ui.Space(1), Right: ui.Space(3), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(cgtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return s.statusGlyph(gtx, on, sv) }),
			ui.Gap(1.5),
			layout.Rigid(on.Text(ui.TextSM, sv.Label).Weight(font.Medium).Layout),
		)
	})
	body := m.Stop()
	gap := gtx.Dp(cardInHandGap)
	x := p.X + gap
	if x+pill.Size.X > size.X {
		x = p.X - gap - pill.Size.X
	}
	at := image.Pt(x, p.Y-pill.Size.Y/2)
	defer op.Offset(at).Push(gtx.Ops).Pop()
	shape := clip.UniformRRect(image.Rectangle{Max: pill.Size}, pill.Size.Y/2)
	paint.FillShape(gtx.Ops, th.Popover, shape.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, th.Ring, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(1))}.Op())
	body.Add(gtx.Ops)
}

// The pill a carried session becomes sits cardInHandGap right of the
// pointer, clear of the cursor, or as far left of it where the window ends
// first, its name cut short past cardInHandWidth.
var (
	cardInHandGap   = unit.Dp(14)
	cardInHandWidth = unit.Dp(240)
)

func (s *sidebar) renaming(id string) bool {
	rename := s.renames[id]
	return rename != nil && rename.Active()
}

// cardBody is the card's one line: its place in the tree, the title row, a
// folded mother's count, the open pull request and the checkout's changes
// unless a checkout row above shows them, then the corner slot.
func (s *sidebar) cardBody(gtx C, th *ui.Theme, sv *sessionView, prChip, closeClick *widget.Clickable, hovered bool) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	var chev layout.Widget
	if sv.place.kids > 0 {
		chev = treeChevronGlyph(th, clickable(s.mothers, sv.ID), !sv.place.folded)
	}
	row := append(treeLead(sv.place, chev), s.titleRow(th, sv)...)
	if sv.place.folded {
		row = append(row, ui.Gap(1.5))
		row = append(row, foldedCount(th, sv.place.kids, sv.place.mark)...)
	}
	if sv.place.hideGit {
		return s.cardLine(gtx, th, sv, closeClick, hovered, row)
	}
	if sv.pr != nil {
		row = append(row, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return s.prBadge(gtx, th, prChip, sv.pr.Number) }))
	}
	if sv.git.Files > 0 {
		row = append(row, ui.Gap(1.5), layout.Rigid(func(gtx C) D { return diffStat(gtx, th, sv.git.Added, sv.git.Deleted) }))
	}
	return s.cardLine(gtx, th, sv, closeClick, hovered, row)
}

// cardLine ends row with the corner slot and lays it out in the card's insets.
func (s *sidebar) cardLine(gtx C, th *ui.Theme, sv *sessionView, closeClick *widget.Clickable, hovered bool, row []layout.FlexChild) D {
	row = append(row, ui.Gap(1), layout.Rigid(func(gtx C) D { return s.cardSlot(gtx, th, sv, closeClick, hovered) }))
	return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// titleRow is the start of the card's line: the status ring around the
// provider mark, a shield on a session that runs sandboxed, a fork on one born
// of a fork, a globe while its browser runs, and the label.
func (s *sidebar) titleRow(th *ui.Theme, sv *sessionView) []layout.FlexChild {
	row := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.statusRing(gtx, th, sv) }),
		ui.Gap(1.5),
	}
	if sv.sandbox.Confined {
		row = append(row, layout.Rigid(func(gtx C) D { return s.icShield.Layout(gtx, 12, th.MutedForeground) }), ui.Gap(1.5))
	}
	if sv.Forked {
		row = append(row, layout.Rigid(func(gtx C) D { return icMenuFork.Layout(gtx, 12, th.MutedForeground) }), ui.Gap(1.5))
	}
	if sv.browser {
		row = append(row, layout.Rigid(func(gtx C) D { return icBrowser.Layout(gtx, 12, th.MutedForeground) }), ui.Gap(1.5))
	}
	if s.renaming(sv.ID) {
		return append(row, layout.Flexed(1, func(gtx C) D { return s.renameField(gtx, th, sv.ID, s.renames[sv.ID]) }))
	}
	return append(row, layout.Flexed(1, th.Text(ui.TextSM, sv.Label).Weight(font.Medium).Layout))
}

// renameField is the label as a field while the card is being renamed.
func (s *sidebar) renameField(gtx C, th *ui.Theme, id string, rename *ui.InlineEdit) D {
	field := th.InlineEdit(rename, ui.TextSM).Weight(font.Medium)
	d, res := field.Layout(gtx)
	if res.Outcome == ui.InlineEditCommitted {
		s.renamed = sessionRename{id: id, label: res.Value}
	}
	return d
}

// cardSlot is the corner every card keeps free, so nothing on its line moves
// when it fills: the × under the pointer, or the pin of a pinned card, which
// has no × because closing is what the pin withholds. Pinning lives in the
// card's menu.
func (s *sidebar) cardSlot(gtx C, th *ui.Theme, sv *sessionView, closeClick *widget.Clickable, hovered bool) D {
	size := gtx.Dp(cardSlotSize)
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	switch {
	case sv.Pinned:
		return layout.Center.Layout(gtx, func(gtx C) D { return s.icPin.LayoutFilled(gtx, 12, th.MutedForeground) })
	case hovered && !s.renaming(sv.ID):
		return layout.Center.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, closeClick, s.icClose, false) })
	}
	return D{Size: gtx.Constraints.Min}
}

// cardSlotSize is the corner slot's side.
var cardSlotSize = unit.Dp(18)

// prBadge is the open pull request's number, a button that brightens on
// hover.
func (s *sidebar) prBadge(gtx C, th *ui.Theme, click *widget.Clickable, number int) D {
	c := th.MutedForeground
	if click.Hovered() {
		c = th.Foreground
	}
	return click.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return s.icPR.Layout(gtx, 12, c) }),
			ui.Gap(1),
			layout.Rigid(th.Text(ui.TextXS, "#"+strconv.Itoa(number)).In(c).Layout),
		)
	})
}

// statusRing is SessionStatusIcon: the provider mark (the agent live in the
// PTY, else the session's kind) in a ring that spins while busy, is amber
// when waiting on the user and emerald when done, faded once that turn is
// read.
func (s *sidebar) statusRing(gtx C, th *ui.Theme, sv *sessionView) D {
	return s.morph.glyph(gtx, glyphSession+sv.ID, func(gtx C) D { return s.statusGlyph(gtx, th, sv) })
}

// neutralRingAlpha is how strongly a ring with no status draws in a tree.
const neutralRingAlpha = 0.35

func (s *sidebar) statusGlyph(gtx C, th *ui.Theme, sv *sessionView) D {
	size := gtx.Dp(18)
	stroke := float32(gtx.Dp(1.25))
	center := f32.Pt(float32(size)/2, float32(size)/2)
	radius := float32(size)/2 - stroke/2
	switch sv.status {
	case "busy", "compacting":
		ui.StrokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.Over(ui.Alpha(th.MutedForeground, 0.25)))
		turn := float32(time.Since(s.started).Seconds()) * 2 * math.Pi
		ui.StrokeArc(gtx, center, radius, stroke, turn, math.Pi/2, th.MutedForeground)
		th.Loop(gtx)
	case "waiting":
		ui.StrokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.ToneWait)
	case "done":
		ring := th.TonePass
		if !sv.unread {
			ring = th.Over(ui.Alpha(ring, 0.3))
		}
		ui.StrokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, ring)
	default:
		// In a tree every ring is where a link ends, so one with no status
		// still draws, neutral.
		if sv.place.tree {
			ui.StrokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.Over(ui.Alpha(th.MutedForeground, neutralRingAlpha)))
		}
	}
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	kind := sv.Kind
	if sv.agent != "" {
		kind = sv.agent
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return providerIcon(kind).Layout(gtx, 11, th.MutedForeground)
	})
}
