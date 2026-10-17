package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/omartelo/lich/native/ui"
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
	click, closeClick, pinClick := clickable(s.cards, sv.ID), clickable(s.closes, sv.ID), clickable(s.pins, sv.ID)
	if click.Clicked(gtx) {
		act.activate = sv.ID
	}
	if closeClick.Clicked(gtx) {
		act.close = sv.ID
	}
	if pinClick.Clicked(gtx) {
		act.pin, act.pinTo = sv.ID, !sv.Pinned
	}
	prChip := clickable(s.prChips, sv.ID)
	if prChip.Clicked(gtx) && sv.pr != nil {
		act.activate, act.openPR = sv.ID, sv.pr.Number
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
	var extra []ui.MenuEntry
	if len(sv.continueIn) > 0 {
		extra = append(extra, continueInItem(th, sv, &act))
	}
	entries := cardMenu(sv, folders, &act, func() { rename.Start(sv.Label) }, func() { s.askFolderName([]string{sv.ID}) }, extra...)
	d := th.ContextMenu(s.root, menu, entries).Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D { return s.cardFace(gtx, th, sv, click, pinClick, closeClick, prChip, fill) })
	})
	if s.renamed.id == sv.ID {
		act.rename, s.renamed = s.renamed, sessionRename{}
	}
	return d, act
}

// cardFace is the card itself, under its tooltip.
// cardFace leaves the corner controls out while the card is being renamed:
// the field takes the whole title line, and a pin or × over it only reads as
// noise next to the text being typed.
func (s *sidebar) cardFace(gtx C, th *ui.Theme, sv *sessionView, click, pinClick, closeClick, prChip *widget.Clickable, fill color.NRGBA) D {
	return click.Layout(gtx, func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, fill, ui.RadiusMD) },
			func(gtx C) D {
				children := []layout.StackChild{
					layout.Stacked(func(gtx C) D { return s.cardBody(gtx, th, sv, prChip) }),
				}
				if !s.renaming(sv.ID) {
					children = append(children, layout.Stacked(func(gtx C) D {
						return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
							return s.cardControls(gtx, th, sv, pinClick, closeClick, click.Hovered())
						})
					}))
				}
				return layout.Stack{Alignment: layout.NE}.Layout(gtx, children...)
			},
		)
	})
}

func (s *sidebar) renaming(id string) bool {
	rename := s.renames[id]
	return rename != nil && rename.Active()
}

func (s *sidebar) cardBody(gtx C, th *ui.Theme, sv *sessionView, prChip *widget.Clickable) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	path := displayPath(sv.shown)
	if sv.host != "" {
		path = unknownCwd(sv.host)
	}
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, s.titleRow(th, sv)...)
		}),
	}
	if rung, ok := statusRung(th, sv); ok {
		rows = append(rows, ui.VGap(2), layout.Rigid(rung))
	}
	rows = append(rows, ui.VGap(2), layout.Rigid(th.Text(ui.TextXS, path).Muted().Mono().LayoutTail))
	if sv.git.Branch != "" || sv.profileName != "" {
		rows = append(rows, ui.VGap(2), layout.Rigid(func(gtx C) D { return s.branchRow(gtx, th, sv, prChip) }))
	}
	return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(2.5), Right: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// titleRow is the card's first line: the status ring around the provider
// mark, a shield on a session that runs sandboxed, and the label, which stops
// short of the corner controls (pr-11, pr-6 for a pinned card's lone pin).
func (s *sidebar) titleRow(th *ui.Theme, sv *sessionView) []layout.FlexChild {
	row := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.statusRing(gtx, th, sv) }),
		ui.Gap(1.5),
	}
	if sv.sandbox.Confined {
		row = append(row, layout.Rigid(func(gtx C) D { return s.icShield.Layout(gtx, 12, th.MutedForeground) }), ui.Gap(1.5))
	}
	if s.renaming(sv.ID) {
		return append(row, layout.Flexed(1, func(gtx C) D { return s.renameField(gtx, th, sv.ID, s.renames[sv.ID]) }))
	}
	controls := ui.Space(11)
	if sv.Pinned {
		controls = ui.Space(6)
	}
	return append(row, layout.Flexed(1, func(gtx C) D {
		return layout.Inset{Right: controls}.Layout(gtx, th.Text(ui.TextSM, sv.Label).Weight(font.Medium).Layout)
	}))
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

// cardControls is the card's top-right corner. At rest it holds how long the
// status has lasted, and the pin of a pinned card; hovered, the pin and the ×
// take its place. A pinned card has no ×: closing is what the pin withholds.
func (s *sidebar) cardControls(gtx C, th *ui.Theme, sv *sessionView, pin, close *widget.Clickable, hovered bool) D {
	var children []layout.FlexChild
	if !hovered && sv.aged {
		children = append(children, layout.Rigid(th.Text(ui.TextXS, formatAge(sv.age)).Muted().Layout))
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(nextAgeChange(sv.age))})
	}
	if hovered || sv.Pinned {
		if len(children) > 0 {
			children = append(children, ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, pin, s.icPin, sv.Pinned) }))
	}
	if hovered && !sv.Pinned {
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D { return cornerButton(gtx, th, close, s.icClose, false) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// branchRow is the card's last line: the branch, and on the right a cluster
// that reads outward from it: where it stands against its base, its open
// pull request, and the checkout's changes.
func (s *sidebar) branchRow(gtx C, th *ui.Theme, sv *sessionView, prChip *widget.Clickable) D {
	git := sv.git
	children := []layout.FlexChild{layout.Flexed(1, func(gtx C) D { return D{Size: gtx.Constraints.Min} })}
	if git.Branch != "" {
		children = []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return s.icBranch.Layout(gtx, 12, th.MutedForeground) }),
			ui.Gap(1),
			layout.Flexed(1, th.Text(ui.TextXS, git.Branch).Muted().Layout),
		}
	}
	if sv.profileName != "" {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return profileChip(gtx, th, sv.profileName) }))
	}
	if standing := standingOf(sv.base); standing.count > 0 {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return s.baseReadout(gtx, th, standing) }))
	}
	if sv.pr != nil {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return s.prBadge(gtx, th, prChip, sv.pr.Number) }))
	}
	if git.Files > 0 {
		children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return diffStat(gtx, th, git.Added, git.Deleted) }))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// baseReadout is ↓N commits behind the base, or ⚠N files that would conflict
// with it, in the waiting tone.
func (s *sidebar) baseReadout(gtx C, th *ui.Theme, b baseStanding) D {
	ic, c := s.icBehind, th.MutedForeground
	if b.conflict {
		ic, c = s.icClash, th.ToneWait
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, c) }),
		ui.Gap(0.5),
		layout.Rigid(th.Text(ui.TextXS, strconv.Itoa(b.count)).In(c).Layout),
	)
}

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

func (s *sidebar) statusGlyph(gtx C, th *ui.Theme, sv *sessionView) D {
	size := gtx.Dp(22)
	stroke := float32(gtx.Dp(1.25))
	center := f32.Pt(float32(size)/2, float32(size)/2)
	radius := float32(size)/2 - stroke/2
	switch sv.status {
	case "busy", "compacting":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.Over(ui.Alpha(th.MutedForeground, 0.25)))
		turn := float32(time.Since(s.started).Seconds()) * 2 * math.Pi
		strokeArc(gtx, center, radius, stroke, turn, math.Pi/2, th.MutedForeground)
		gtx.Execute(op.InvalidateCmd{})
	case "waiting":
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, th.ToneWait)
	case "done":
		ring := th.TonePass
		if !sv.unread {
			ring = th.Over(ui.Alpha(ring, 0.3))
		}
		strokeArc(gtx, center, radius, stroke, 0, 2*math.Pi, ring)
	}
	gtx.Constraints = layout.Exact(image.Pt(size, size))
	kind := sv.Kind
	if sv.agent != "" {
		kind = sv.agent
	}
	return layout.Center.Layout(gtx, func(gtx C) D {
		return providerIcon(kind).Layout(gtx, 14, th.MutedForeground)
	})
}

func strokeArc(gtx C, center f32.Point, radius, width, start, sweep float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	from := f32.Pt(center.X+radius*float32(math.Cos(float64(start))), center.Y+radius*float32(math.Sin(float64(start))))
	p.MoveTo(from)
	p.ArcTo(center, center, sweep)
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}
