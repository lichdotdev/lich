package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// rowsState is the live state of the text lines, list rows and faces.
type rowsState struct {
	links      [3]widget.Clickable
	checks     [2]widget.Bool
	busyButton widget.Clickable
	navRows    [3]widget.Clickable
	navOn      int
	panelRows  [2]widget.Clickable
	ghostTexts [2]widget.Clickable
	cardIcons  [2]widget.Clickable
	cardTurn   float32
	iconBtns   [2]widget.Clickable
	iconTips   [2]ui.Tooltip
	trigger    widget.Clickable
	triggerOn  bool
	faceOn     [2]widget.Clickable
}

// Measures the product hands the rows; the values are the ones lich-native uses.
var (
	navRowHeight = ui.Space(8)
	navRowPadX   = ui.Space(1.5)
	panelEdge    = ui.Space(2)
	panelRowPadX = ui.Space(1.5)
	cardBtnSide  = ui.Space(6)
)

func (g *gallery) glyphsSection(gtx C, th *ui.Theme) D {
	ic := g.icon
	return ui.Stack(4,
		caption(th, "IconLine: glyph, gap-1.5, any children"),
		ui.Stack(1,
			func(gtx C) D {
				return ui.IconLine(gtx, ic("git-branch"), th.MutedForeground, layout.Rigid(th.Text(ui.TextXS, "feature/toolbar").Muted().Layout))
			},
			func(gtx C) D {
				return ui.IconLine(gtx, ic("shield"), th.Foreground, layout.Rigid(th.Text(ui.TextXS, "Sandboxed").Layout))
			},
		),
		caption(th, "IconLabel: glyph, gap-1, label filling the line"),
		minWidth(220, ui.IconLabel(ic("hourglass"), th.ToneWait, th.Text(ui.TextXS, "Waiting on you").In(th.ToneWait)).Layout),
		caption(th, "MonoGlyph: MetaGlyph in the mono face"),
		ui.Row(4,
			th.MonoGlyph(ic("git-branch"), "feature/toolbar", th.MutedForeground).Layout,
			th.MonoGlyph(ic("git-branch"), "main", th.Foreground).Layout,
		),
		caption(th, "MutedGlyph: 16 and 12"),
		ui.Row(4, th.MutedGlyph(ic("folder"), 16).Layout, th.MutedGlyph(ic("message-square"), 12).Layout),
		caption(th, "CenteredIcon in a 22x22 box, BoxIcon in 22x22 and 14x14"),
		ui.Row(4,
			box(22, 22, func(gtx C) D { return ui.CenteredIcon(gtx, ic("git-branch"), 13, th.Foreground) }),
			box(22, 22, func(gtx C) D { return ui.BoxIcon(gtx, ic("folder"), th.Foreground) }),
			box(14, 14, func(gtx C) D { return ui.BoxIcon(gtx, ic("folder"), th.Foreground) }),
		),
		caption(th, "TriggerChevron: closed, open (click the field)"),
		ui.Row(4,
			func(gtx C) D { return th.TriggerChevron(gtx, false) },
			func(gtx C) D { return th.TriggerChevron(gtx, true) },
			func(gtx C) D {
				s := &g.rowsState
				if s.trigger.Clicked(gtx) {
					s.triggerOn = !s.triggerOn
				}
				return s.trigger.Layout(gtx, func(gtx C) D { return th.TriggerChevron(gtx, s.triggerOn) })
			},
		),
		caption(th, "TinyChip: wait, muted, pass"),
		ui.Row(3,
			th.TinyChip("waiting", th.ToneWait).Layout,
			th.TinyChip("required", th.MutedForeground).Layout,
			th.TinyChip("gone", th.TonePass).Layout,
		),
	)(gtx)
}

func (g *gallery) linesSection(gtx C, th *ui.Theme) D {
	s := &g.rowsState
	b := th.Button(&s.busyButton, "Check for updates")
	b.Size = ui.SizeSM
	return ui.Stack(4,
		caption(th, "HoverLink: rest, hover"),
		func(gtx C) D { return th.HoverLink(gtx, &s.links[0], "Pull requests") },
		caption(th, "TrailingIconLink"),
		func(gtx C) D { return th.TrailingIconLink(gtx, &s.links[1], "Documentation", g.icon("external-link")) },
		caption(th, "TwoLines"),
		th.TwoLines("feature/toolbar", "~/code/lich/native").Layout,
		caption(th, "SpinnerLine"),
		func(gtx C) D { return th.SpinnerLine(gtx, "Waiting for the browser…") },
		caption(th, "SpinnerBeside: a button while it works"),
		func(gtx C) D { return th.SpinnerBeside(gtx, b.Layout) },
		caption(th, "CheckRow: label only, with a note"),
		minWidth(300, func(gtx C) D { return th.CheckRow(gtx, &s.checks[0], "Run confined", "") }),
		minWidth(300, func(gtx C) D {
			return th.CheckRow(gtx, &s.checks[1], "Carry my changes", "Uncommitted work moves to the new checkout.")
		}),
	)(gtx)
}

func (g *gallery) listRowsSection(gtx C, th *ui.Theme) D {
	s := &g.rowsState
	item := func(i int, label string) layout.Widget {
		return func(gtx C) D {
			if s.navRows[i].Clicked(gtx) {
				s.navOn = i
			}
			return th.NavRow(gtx, &s.navRows[i], s.navOn == i, navRowHeight, navRowPadX, func(gtx C, th *ui.Theme) D {
				return th.Text(ui.TextSM, label).Layout(gtx)
			})
		}
	}
	panel := func(i int, live bool, label string) layout.Widget {
		return func(gtx C) D {
			return th.PanelRow(gtx, &s.panelRows[i], live, panelEdge, panelRowPadX, func(gtx C, th *ui.Theme) D {
				return th.Text(ui.TextSM, label).Layout(gtx)
			})
		}
	}
	return ui.Stack(4,
		caption(th, "NavRow: rest, hover, on (click to move it)"),
		ui.Stack(0, item(0, "General"), item(1, "Appearance"), item(2, "Hotkeys")),
		caption(th, "PanelRow: live (hover it), not live"),
		ui.Stack(0, panel(0, true, "Lint · passed"), panel(1, false, "Deploy · no link")),
		caption(th, "FillRow: accent at 0.4 and 1"),
		ui.Row(3,
			func(gtx C) D {
				th.FillRow(gtx, image.Pt(gtx.Dp(120), gtx.Dp(28)), 0.4)
				return D{Size: image.Pt(gtx.Dp(120), gtx.Dp(28))}
			},
			func(gtx C) D {
				th.FillRow(gtx, image.Pt(gtx.Dp(120), gtx.Dp(28)), 1)
				return D{Size: image.Pt(gtx.Dp(120), gtx.Dp(28))}
			},
		),
		caption(th, "MenuHeader, MenuSeparator"),
		minWidth(260, func(gtx C) D {
			return ui.Stack(0,
				func(gtx C) D { return th.MenuHeader(gtx, "Notifications", "3") },
				th.MenuSeparator,
				func(gtx C) D { return th.MenuHeader(gtx, "Recent", "") },
			)(gtx)
		}),
		caption(th, "RailHairline"),
		th.RailHairline,
		caption(th, "CaptionDivider (inset 6, height 20)"),
		func(gtx C) D { return th.CaptionDivider(gtx, unit.Dp(6), unit.Dp(20)) },
		caption(th, "OrRule"),
		minWidth(300, func(gtx C) D { return th.OrRule(gtx, "or") }),
	)(gtx)
}

func (g *gallery) facesSection(gtx C, th *ui.Theme) D {
	s := &g.rowsState
	tip := func(i int, hint string, disabled bool) layout.Widget {
		return func(gtx C) D {
			return th.GhostIconButton(gtx, &g.root, &s.iconBtns[i], &s.iconTips[i], g.icon("rotate-ccw"), hint, disabled)
		}
	}
	icon := func(gtx C) D { return g.icon("folder").Layout(gtx, ui.Space(4), th.MutedForeground) }
	return ui.Stack(4,
		caption(th, "GhostTextButton: rest, hover, destructive ink"),
		ui.Row(3,
			func(gtx C) D { return th.GhostTextButton(gtx, &s.ghostTexts[0], "Remove", th.Foreground) },
			func(gtx C) D { return th.GhostTextButton(gtx, &s.ghostTexts[1], "Remove", th.Destructive) },
		),
		caption(th, "CardIconButton: rest, turned, CardIconDisabled"),
		ui.Row(3,
			func(gtx C) D { return th.CardIconButton(gtx, &s.cardIcons[0], g.icon("refresh-cw"), cardBtnSide, 0) },
			func(gtx C) D { return th.CardIconButton(gtx, &s.cardIcons[1], g.icon("refresh-cw"), cardBtnSide, 1.2) },
			func(gtx C) D { return th.CardIconDisabled(gtx, g.icon("refresh-cw"), cardBtnSide, 0) },
		),
		caption(th, "GhostIconButton: tooltip, disabled"),
		ui.Row(3, tip(0, "Reset to default", false), tip(1, "Nothing to reset", true)),
		caption(th, "SquareFace: rest, Accent"),
		ui.Row(3,
			func(gtx C) D { return ui.SquareFace(gtx, th.Surface, icon) },
			func(gtx C) D { return ui.SquareFace(gtx, th.Accent, icon) },
		),
		caption(th, "LabelFace: rest, Accent"),
		minWidth(220, func(gtx C) D { return th.LabelFace(gtx, th.Surface, icon, "New session") }),
		minWidth(220, func(gtx C) D { return th.LabelFace(gtx, th.Accent, icon, "New session") }),
	)(gtx)
}

// box is w in a fixed w x h box.
func box(w, h int, widget layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(w)), gtx.Dp(unit.Dp(h))))
		return widget(gtx)
	}
}
