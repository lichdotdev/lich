package main

import (
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
)

// contentState is the live state of the small content widgets: the kit's
// primitives keep none of their own.
type contentState struct {
	iconActions [2]ui.IconActionState
	corners     [2]widget.Clickable
	bars        [3]widget.Clickable
	popoverRows [2]widget.Clickable
}

// panelPadX is the side padding a screen hands PanelNotice: the Pulls body's
// 14px.
var panelPadX = ui.Space(3.5)

func (g *gallery) noticesSection(gtx C, th *ui.Theme) D {
	return ui.Stack(4,
		caption(th, "Notice"),
		th.Notice("Nothing here yet. A panel says so in one quiet line.").Layout,
		caption(th, "Footnote (under a list)"),
		ui.Stack(0,
			th.Text(ui.TextSM, "feature/toolbar").Layout,
			th.Text(ui.TextSM, "feature/dock").Layout,
			th.Footnote("Showing the 2 branches changed in the last 30 days.").Layout,
		),
		caption(th, "PanelNotice (text-sm, set in by the screen's padding)"),
		th.PanelNotice("Could not read the pull request.", panelPadX).Layout,
		caption(th, "BodyLine"),
		th.BodyLine("Only whitespace changed in this file.").Layout,
	)(gtx)
}

func (g *gallery) iconButtonsSection(gtx C, th *ui.Theme) D {
	s := &g.contentState
	root := &g.root
	return ui.Stack(4,
		caption(th, "IconAction: rest, pressed (hover the first one for its tooltip)"),
		ui.Row(3,
			th.IconAction(root, &s.iconActions[0], g.icon("columns-2"), "Split view", false).Layout,
			th.IconAction(root, &s.iconActions[1], g.icon("columns-2"), "Split view, on", true).Layout,
		),
		caption(th, "CornerButton: rest, On"),
		ui.Row(3,
			th.CornerButton(&s.corners[0], g.icon("ellipsis"), false).Layout,
			th.CornerButton(&s.corners[1], g.icon("pin"), true).Layout,
		),
		caption(th, "BarIconButton: rest, On (popover open), disabled (nil click)"),
		ui.Row(3,
			th.BarIconButton(&s.bars[0], g.icon("settings"), false).Layout,
			th.BarIconButton(&s.bars[1], g.icon("bell"), true).Layout,
			th.BarIconButton(nil, g.icon("plus"), false).Layout,
		),
	)(gtx)
}

func (g *gallery) metaSection(gtx C, th *ui.Theme) D {
	swatch := func(gtx C) D { return ui.Swatch(gtx, ui.Dark.Primary) }
	return ui.Stack(4,
		caption(th, "DiffStat"),
		ui.Row(6, th.DiffStat(12, 3).Layout, th.DiffStat(0, 41).Layout, th.DiffStat(0, 0).Layout),
		caption(th, "MetaGlyph: foreground, muted, wait, pass"),
		ui.Row(4,
			th.MetaGlyph(g.icon("git-branch"), "main", th.Foreground).Layout,
			th.MetaGlyph(g.icon("git-branch"), "feature/toolbar", th.MutedForeground).Layout,
			th.MetaGlyph(g.icon("circle-dot"), "Changes requested", th.ToneWait).Layout,
			th.MetaGlyph(g.icon("check"), "Resolved", th.TonePass).Layout,
		),
		caption(th, "Pill: with a mark, text only"),
		ui.Row(3,
			th.Pill(ui.Row(2, swatch, th.Text(ui.TextXS, "lich").Muted().Layout), ui.Space(1.75), ui.Space(2.25)).Layout,
			th.Pill(th.Text(ui.TextXS, "Global only").Muted().Layout, ui.Space(2), ui.Space(2)).Layout,
		),
		caption(th, "KeyCap"),
		ui.Row(3, th.KeyCap("Esc").Layout, th.KeyCap("Ctrl+K").Layout, th.KeyCap("Ctrl+Shift+P").Layout),
	)(gtx)
}

func (g *gallery) rowsSection(gtx C, th *ui.Theme) D {
	s := &g.contentState
	label := func(text string) func(C, *ui.Theme) D {
		return func(gtx C, th *ui.Theme) D { return th.Text(ui.TextSM, text).Layout(gtx) }
	}
	trigger := th.SelectTrigger("origin/main", 240)
	led := th.SelectTrigger("feature/toolbar", 240)
	led.Lead = func(gtx C) D { return g.icon("git-branch").Layout(gtx, ui.Space(4), th.MutedForeground) }
	return ui.Stack(4,
		caption(th, "PopoverRow (hover the second one)"),
		ui.Stack(0,
			th.PopoverRow(&s.popoverRows[0], label("Open in editor")).Layout,
			th.PopoverRow(&s.popoverRows[1], label("Copy path")).Layout,
		),
		caption(th, "SelectTrigger: plain, with a lead mark"),
		ui.Stack(2, trigger.Layout, led.Layout),
		caption(th, "RuleAbove (the same pixels as Separator, drawn with border-t)"),
		th.RuleAbove,
		caption(th, "VerticalRule"),
		height(24, ui.Row(3, th.Text(ui.TextSM, "Files").Layout, th.VerticalRule, th.Text(ui.TextSM, "Diff").Layout)),
	)(gtx)
}
