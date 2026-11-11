package main

import (
	"image"
	"image/color"
	"slices"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// coverageState is the state of every widget the coverage sections add: the
// kit keeps none of its own, so the gallery does, as in sections.go.
type coverageState struct {
	ready bool

	inline      ui.InlineEdit
	inlineName  string
	inlineLast  string
	inlineStart widget.Clickable

	fields   [6]widget.Editor
	clear    widget.Clickable
	areas    [4]widget.Editor
	groups   [5]widget.Editor
	groupBtn widget.Clickable

	tips        [2]ui.Tooltip
	tipTriggers [2]widget.Clickable
	panels      [2]ui.DropdownPanelState
	panelTrig   [2]widget.Clickable
	menu        ui.MenuState
	radio       string
	menuLast    string

	toastButtons [3]widget.Clickable

	dialogs      [4]ui.DialogState
	dialogOpen   int // 1-based index into dialogs; 0 is none
	dialogOpener [4]widget.Clickable
	dialogClose  widget.Clickable
	dialogField  widget.Editor
	dialogPick   widget.Editor

	resize      ui.ResizeHandle
	resizeWidth unit.Dp
	toggles     [2]widget.Bool
	groupSmall  widget.Enum
	tabs        ui.TabsState

	sortV, sortH, sortS ui.Sortable
	idsV, idsH, idsS    []string
}

const (
	coverageResizeMin, coverageResizeMax, coverageResizeStart unit.Dp = 96, 240, 140
	coverageSortPinned                                                = "Pinned"
)

func (c *coverageState) init() {
	if c.ready {
		return
	}
	c.ready = true
	c.inlineName = "Refactor the billing module"
	c.inlineLast = "none yet"
	c.fields[0].SetText("git rebase --onto main")
	c.fields[1].SetText("Borderless inside a panel")
	c.fields[3].SetText("Clear me")
	c.areas[1].SetText("Locked text")
	c.areas[2].SetText("Not a valid prompt")
	c.groups[0].SetText("not-a-url")
	c.groups[2].SetText("lich")
	c.radio = "system"
	c.menuLast = "none yet"
	c.groupSmall.Value = "b"
	c.resizeWidth = coverageResizeStart
	c.resize.Edge = ui.PanelEdgeLeft
	c.idsV = []string{"Plan", "Build", "Review", "Ship"}
	c.idsH = []string{"main", "feature/a", "fix/b", "docs"}
	c.idsS = []string{coverageSortPinned, "Second", "Third", "Fourth"}
}

// coverage is the page's last block: every kit part and variant the sections
// above leave out, in two columns like the page.
func (g *gallery) coverage(gtx C) D {
	th := g.th
	g.cov.init()
	return ui.Stack(6,
		th.Text(ui.TextBase, "Coverage").Weight(font.Medium).Layout,
		th.Text(ui.TextSM, "The rest of the kit: each variant, state and part the sections above do not draw.").Muted().Layout,
		func(gtx C) D {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Flexed(1, ui.Stack(6,
					g.section("InlineEdit", g.inlineSection),
					g.section("Progress, Spinner, BusyRing, Reveal", g.motionSection),
					g.section("Input, Textarea variants", g.fieldVariantsSection),
					g.section("InputGroup variants", g.groupVariantsSection),
					g.section("Tooltip sides, DropdownPanel", g.popupVariantsSection),
					g.section("Menu entries, ContextMenu", g.menuVariantsSection),
				)),
				ui.Gap(6),
				layout.Flexed(1, ui.Stack(6,
					g.section("Toast: secondary, icon, lines", g.toastVariantsSection),
					g.section("Dialog variants", g.dialogVariantsSection),
					g.section("ResizeHandle left, Toggle, ToggleGroup, Tabs", g.controlVariantsSection),
					g.section("SortList, Skeleton", g.sortSection),
					g.section("Label variants", g.labelVariantsSection),
					g.section("Icons: filled, Sonner, LichMark, Devicon", g.iconVariantsSection),
					g.section("Palette", g.paletteSection),
				)),
			)
		},
	)(gtx)
}

func labeled(th *ui.Theme, label string, w layout.Widget) layout.Widget {
	return ui.Stack(1.5, caption(th, label), w)
}

// capped limits a widget to width, as a w-* does on a block.
func capped(width unit.Dp, w layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(width))
		return w(gtx)
	}
}

// fullWidth is a block's w-full for a widget the section's stack would size to
// its content, as text-center needs.
func fullWidth(w layout.Widget) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	}
}

func (g *gallery) inlineSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	if c.inlineStart.Clicked(gtx) {
		c.inline.Start(c.inlineName)
	}
	name := func(gtx C) D {
		if !c.inline.Active() {
			return th.Text(ui.TextSM, c.inlineName).Weight(font.Medium).Layout(gtx)
		}
		d, res := th.InlineEdit(&c.inline, ui.TextSM).Weight(font.Medium).Layout(gtx)
		switch res.Outcome {
		case ui.InlineEditCommitted:
			c.inlineName, c.inlineLast = res.Value, "committed "+res.Value
		case ui.InlineEditCancelled:
			c.inlineLast = "cancelled"
		}
		return d
	}
	rename := th.Button(&c.inlineStart, "Rename")
	rename.Variant, rename.Size, rename.Icon = ui.VariantOutline, ui.SizeSM, g.icon("pencil")
	return ui.Stack(3,
		caption(th, "Enter or blur commits, Escape or an unchanged value cancels"),
		capped(280, name),
		rename.Layout,
		th.Text(ui.TextXS, "Last outcome: "+c.inlineLast).Muted().Layout,
	)(gtx)
}

func (g *gallery) motionSection(gtx C, th *ui.Theme) D {
	percents := []int{0, 35, 100, -1}
	names := []string{"0%", "35%", "100%", "indeterminate (negative percent)"}
	var bars []layout.Widget
	for i, p := range percents {
		bars = append(bars, labeled(th, names[i], th.Progress(p).Layout))
	}
	spin := func(size unit.Dp, fg color.NRGBA) layout.Widget {
		return func(gtx C) D { return th.Spinner(gtx, size, fg) }
	}
	ring := func(size unit.Dp, fg color.NRGBA) layout.Widget {
		return func(gtx C) D { return th.BusyRing(gtx, size, fg, th.RingTrack(fg)) }
	}
	box := func(rows float32) layout.Widget {
		return func(gtx C) D {
			return ui.Reveal(gtx, rows, func(gtx C) D { return g.revealBox(gtx, th) })
		}
	}
	return ui.Stack(4,
		ui.Stack(2.5, bars...),
		caption(th, "Spinner: size 4, 6, muted"),
		ui.Row(4, spin(ui.Space(4), th.Foreground), spin(ui.Space(6), th.Foreground), spin(ui.Space(6), th.MutedForeground)),
		caption(th, "BusyRing: size 4, 6, TonePass"),
		ui.Row(4, ring(ui.Space(4), th.Foreground), ring(ui.Space(6), th.Foreground), ring(ui.Space(6), th.TonePass)),
		caption(th, "Reveal at 0.25, 0.5 and 1 of the height"),
		ui.Columns(3, box(0.25), box(0.5), box(1)),
	)(gtx)
}

// revealBox is Reveal's content: three lines on a muted fill, so the clip
// shows against the card.
func (g *gallery) revealBox(gtx C, th *ui.Theme) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Over(ui.Alpha(th.Muted, 0.6)), ui.RadiusMD) },
		func(gtx C) D {
			return layout.UniformInset(ui.Space(2)).Layout(gtx, ui.Stack(1,
				th.Text(ui.TextXS, "First line").Layout,
				th.Text(ui.TextXS, "Second line").Layout,
				th.Text(ui.TextXS, "Third line").Layout,
			))
		},
	)
}

func (g *gallery) fieldVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	field := func(i int, placeholder string) ui.InputStyle { return th.Input(&c.fields[i], placeholder) }
	mono := field(0, "Command")
	mono.Mono = true
	borderless := field(1, "Borderless")
	borderless.Borderless = true
	bare := field(2, "Bare: no box at all")
	bare.Bare = true
	clearable := field(3, "Type, then clear")
	clearable.Clear = &c.clear
	narrow := field(4, "w-40")
	narrow.Width = 160
	leading := field(5, "Leading size 5")
	leading.Leading, leading.LeadingSize = g.icon("search"), ui.Space(5)

	resizeNone := th.Textarea(&c.areas[0], "Resize nil: resize-none, grows with its text")
	disabled := th.Textarea(&c.areas[1], "Disabled")
	disabled.Disabled = true
	invalid := th.Textarea(&c.areas[2], "Invalid")
	invalid.Invalid = true
	narrowArea := th.Textarea(&c.areas[3], "Width 240")
	narrowArea.Width = 240
	return ui.Stack(4,
		ui.Columns(4, labeled(th, "Input, Mono", mono.Layout), labeled(th, "Borderless", borderless.Layout)),
		ui.Columns(4, labeled(th, "Bare", bare.Layout), labeled(th, "Clear (X while it has text)", clearable.Layout)),
		ui.Columns(4, labeled(th, "Width", narrow.Layout), labeled(th, "LeadingSize", leading.Layout)),
		labeled(th, "Textarea, Resize nil", resizeNone.Layout),
		ui.Columns(4, labeled(th, "Textarea, Disabled", disabled.Layout), labeled(th, "Textarea, Invalid", invalid.Layout)),
		labeled(th, "Textarea, Width", narrowArea.Layout),
	)(gtx)
}

func (g *gallery) groupVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	invalid := th.InputGroup(&c.groups[0], "Repository URL")
	invalid.Invalid = true
	invalid.Start = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(g.icon("link"))}}
	narrow := th.InputGroup(&c.groups[1], "w-60")
	narrow.Width = 240
	narrow.Start = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(g.icon("search"))}}
	endText := th.InputGroup(&c.groups[2], "Workspace")
	endText.End = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupText(".lich.dev")}}
	endIcon := th.InputGroup(&c.groups[3], "Branch")
	endIcon.End = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(g.icon("git-branch"))}}

	area := th.InputGroupTextarea(&c.groups[4], "Message the agent…")
	send := area.Inner().InputGroupButton(&c.groupBtn, "")
	send.Icon, send.Size = g.icon("arrow-up"), ui.SizeIconXS
	area.Start = ui.InputGroupAddon{Items: []layout.Widget{th.InputGroupIcon(g.icon("paperclip"))}}
	area.End = ui.InputGroupAddon{Items: []layout.Widget{send.Layout}, Button: true}
	return ui.Stack(4,
		labeled(th, "InputGroup, Invalid", invalid.Layout),
		labeled(th, "InputGroup, Width", narrow.Layout),
		labeled(th, "End addon, text", endText.Layout),
		labeled(th, "End addon, icon", endIcon.Layout),
		labeled(th, "InputGroupTextarea, icon start and button end", area.Layout),
	)(gtx)
}

func (g *gallery) popupVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	sideTip := func(i int, side ui.Side, label string) layout.Widget {
		trigger := th.Button(&c.tipTriggers[i], label)
		trigger.Variant = ui.VariantOutline
		tip := th.Tooltip(&g.root, &c.tips[i], "Opens to the "+label)
		tip.Side = side
		return func(gtx C) D { return tip.Layout(gtx, trigger.Layout) }
	}
	panel := func(i int, align bool, label string) layout.Widget {
		trigger := th.Button(&c.panelTrig[i], label)
		trigger.Variant, trigger.Icon = ui.VariantOutline, g.icon("chevron-down")
		p := th.DropdownPanel(&g.root, &c.panels[i], ui.Space(60), g.panelRows)
		p.AlignEnd = align
		return func(gtx C) D { return p.Layout(gtx, trigger.Layout) }
	}
	spread := func(left, right layout.Widget) layout.Widget {
		return func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(left),
				layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, 0)} }),
				layout.Rigid(right),
			)
		}
	}
	return ui.Stack(4,
		caption(th, "Tooltip, SideLeft and SideRight (hover)"),
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(36), Right: ui.Space(36)}.Layout(gtx,
				ui.Row(2, sideTip(0, ui.SideLeft, "left"), sideTip(1, ui.SideRight, "right")))
		},
		caption(th, "DropdownPanel: left edge aligned, then AlignEnd"),
		spread(panel(0, false, "Align start"), panel(1, true, "Align end")),
	)(gtx)
}

// panelRows is DropdownPanel's content: two-line rows no MenuEntry can say.
func (g *gallery) panelRows(gtx C, th *ui.Theme) D {
	item := func(title, sub string) layout.Widget {
		return func(gtx C) D {
			return layout.UniformInset(ui.Space(2)).Layout(gtx, ui.Stack(0.5,
				th.Text(ui.TextSM, title).Layout,
				th.Text(ui.TextXS, sub).Muted().Layout,
			))
		}
	}
	return ui.Stack(0,
		item("claude · opus", "12 turns, 48k tokens"),
		item("codex · gpt", "3 turns, 9k tokens"),
	)(gtx)
}

func (g *gallery) menuVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	pick := func(v string) func() { return func() { c.radio, c.menuLast = v, "Theme "+v } }
	avatar := func(gtx C, size unit.Dp) D {
		gtx.Constraints.Min = image.Pt(gtx.Dp(size), gtx.Dp(size))
		return ui.Fill(gtx, th.Primary, size/2)
	}
	radio := func(v, label string) ui.MenuEntry {
		return ui.MenuEntry{Kind: ui.MenuRadioItem, Label: label, Checked: c.radio == v, OnClick: pick(v)}
	}
	entries := []ui.MenuEntry{
		{Kind: ui.MenuLabel, Label: "Appearance"},
		radio("light", "Light"), radio("dark", "Dark"), radio("system", "System"),
		{Kind: ui.MenuSeparator},
		{Label: "Signed in as Ana", Lead: avatar},
		{Label: "Inset: aligned under the checkable rows", Inset: true},
		{Label: "Disabled row", Icon: g.icon("lock"), Disabled: true},
		{Label: "Disabled with shortcut", Shortcut: "Ctrl+Z", Disabled: true},
		{Kind: ui.MenuSeparator},
		{Label: "Delete everything", Icon: g.icon("trash-2"), Shortcut: "No undo", ShortcutDestructive: true,
			Variant: ui.VariantDestructive, OnClick: func() { c.menuLast = "Delete everything" }},
	}
	menu := th.ContextMenu(&g.root, &c.menu, entries)
	return ui.Stack(4,
		caption(th, "ContextMenu: radio, Lead, Inset, Disabled, ShortcutDestructive; right-click the area"),
		func(gtx C) D {
			return menu.Layout(gtx, func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				gtx.Constraints.Min.Y = gtx.Dp(ui.Space(16))
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, th.Over(ui.Alpha(th.Muted, 0.5)), ui.RadiusMD) },
					func(gtx C) D {
						return layout.Center.Layout(gtx, th.Text(ui.TextSM, "Right-click: it opens at the pointer").Muted().Layout)
					},
				)
			})
		},
		th.Text(ui.TextXS, "Theme: "+c.radio+". Last action: "+c.menuLast).Muted().Layout,
	)(gtx)
}

func (g *gallery) toastVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	demos := []struct {
		label string
		toast ui.Toast
	}{
		{"Secondary button", ui.Toast{Title: "lich 1.4.0 is ready", Description: "Restart to finish installing it.",
			Duration: ui.ToastForever, Cancel: &ui.ToastButton{Label: "Later"},
			Secondary: &ui.ToastButton{Label: "What's new"}, Action: &ui.ToastButton{Label: "Restart"}}},
		{"Icon, IconColor", ui.Toast{Title: "Pushed feature/toolbar", Icon: g.icon("git-branch"),
			IconColor: func(th *ui.Theme) color.NRGBA { return th.TonePass }}},
		{"Lines", ui.Toast{Title: "Reviewing 3 files", Lines: []ui.ToastLine{
			{Icon: g.icon("file"), Text: "ui/inline_edit.go"}, {Text: "ui/progress.go, no glyph"}}}},
	}
	var buttons []layout.Widget
	for i, demo := range demos {
		if c.toastButtons[i].Clicked(gtx) {
			g.raise(demo.toast)
		}
		b := th.Button(&c.toastButtons[i], demo.label)
		b.Variant = ui.VariantOutline
		buttons = append(buttons, b.Layout)
	}
	return ui.Stack(4, caption(th, "Raised on the page's Toaster"), ui.Row(2, buttons...))(gtx)
}

func (g *gallery) dialogVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	labels := []string{"Body", "HideClose", "Raw, Top, Scrim", "DialogWidthXL"}
	var buttons []layout.Widget
	for i, label := range labels {
		if c.dialogOpener[i].Clicked(gtx) {
			c.dialogOpen = i + 1
		}
		b := th.Button(&c.dialogOpener[i], label)
		b.Variant = ui.VariantOutline
		buttons = append(buttons, b.Layout)
	}
	return ui.Stack(4, caption(th, "Each opens its own dialog"), ui.Row(2, buttons...))(gtx)
}

// layoutCoverageDialogs lays out the coverage section's dialogs. Like
// layoutDialog it runs before the page: a dialog is window-sized and claims
// Escape ahead of everything under it.
func (g *gallery) layoutCoverageDialogs(gtx C) {
	c := &g.cov
	c.init()
	if c.dialogClose.Clicked(gtx) {
		c.dialogOpen = 0
	}
	for i := range c.dialogs {
		if g.coverageDialog(i).Layout(gtx) && c.dialogOpen == i+1 {
			c.dialogOpen = 0
		}
	}
}

func (g *gallery) coverageDialog(i int) ui.DialogStyle {
	c := &g.cov
	th := g.th.On(g.th.Popover)
	closer := th.Button(&c.dialogClose, "Close")
	closer.Variant = ui.VariantOutline
	open := c.dialogOpen == i+1
	switch i {
	case 0:
		d := th.Dialog(&c.dialogs[i], open, "Rename session", "Body sits between the header and the footer.")
		field := th.Input(&c.dialogField, "Session name")
		d.Body, d.Actions = field.Layout, []layout.Widget{closer.Layout}
		return d
	case 1:
		d := th.Dialog(&c.dialogs[i], open, "Update required", "HideClose drops the corner ×: only the footer closes it.")
		d.HideClose, d.Actions = true, []layout.Widget{closer.Layout}
		return d
	case 2:
		d := th.Dialog(&c.dialogs[i], open, "", "")
		d.Top, d.Scrim = 0.2, 0.5
		d.Raw = func(gtx C) D { return g.rawDialogPanel(gtx, th) }
		return d
	}
	d := th.Dialog(&c.dialogs[i], open, "A wider dialog", "DialogWidthXL is 576dp where the default is 448dp.")
	d.MaxWidth, d.Actions = ui.DialogWidthXL, []layout.Widget{closer.Layout}
	return d
}

// rawDialogPanel is a Raw dialog's whole panel: a bare field over rows, as a
// picker draws it.
func (g *gallery) rawDialogPanel(gtx C, th *ui.Theme) D {
	field := th.Input(&g.cov.dialogPick, "Raw: no header, footer or ×")
	field.Bare = true
	return layout.UniformInset(ui.Space(2)).Layout(gtx, ui.Stack(1,
		field.Layout,
		th.Separator,
		th.Text(ui.TextSM, "Top places the panel 20% down; Scrim darkens the window").Muted().Layout,
	))
}

func (g *gallery) controlVariantsSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	large := th.Toggle(&c.toggles[0], "Large")
	large.Size = ui.ToggleSizeLG
	largeIcon := th.Toggle(&c.toggles[1], "Large outline")
	largeIcon.Size, largeIcon.Variant, largeIcon.Icon = ui.ToggleSizeLG, ui.ToggleVariantOutline, g.icon("bold")

	group := th.ToggleGroup(&c.groupSmall,
		ui.ToggleItem{Value: "a", Text: "Day"},
		ui.ToggleItem{Value: "b", Text: "Week"},
		ui.ToggleItem{Value: "c", Text: "Month"},
	)
	group.Variant, group.ItemHeight, group.ItemPadX, group.ItemText = ui.ToggleVariantOutline, ui.Space(7), ui.Space(1.5), ui.TextXS

	count := th.Text(ui.TextXS, "12").Muted().Layout
	tabs := th.Tabs(&c.tabs,
		ui.TabsTrigger{Text: "Files", Trailing: count},
		ui.TabsTrigger{Text: "Changes", Trailing: count},
		ui.TabsTrigger{Text: "Checks"},
	)
	panel := th.Text(ui.TextSM, "A trailing widget sits after the trigger's text").Muted().Layout
	return ui.Stack(4,
		caption(th, "ResizeHandle, PanelEdgeLeft: drag the left edge of the panel"),
		fullWidth(g.leftPanel(th)),
		caption(th, "Toggle, ToggleSizeLG"),
		ui.Row(2, large.Layout, largeIcon.Layout),
		caption(th, "ToggleGroup, ItemHeight, ItemPadX and ItemText overrides"),
		group.Layout,
		caption(th, "Tabs, Trailing"),
		height(88, func(gtx C) D { return tabs.Layout(gtx, panel, panel, panel) }),
	)(gtx)
}

// leftPanel is a panel anchored on the right whose handle sits on its left
// edge and grows it leftward.
func (g *gallery) leftPanel(th *ui.Theme) layout.Widget {
	c := &g.cov
	return func(gtx C) D {
		if w, _ := c.resize.Update(gtx, c.resizeWidth, coverageResizeMin, coverageResizeMax); w != c.resizeWidth {
			c.resizeWidth = w
		}
		size := image.Pt(min(gtx.Dp(c.resizeWidth), gtx.Constraints.Max.X), gtx.Dp(ui.Space(24)))
		full := gtx.Constraints.Max.X
		d := layout.E.Layout(gtx, func(gtx C) D {
			gtx.Constraints = layout.Exact(size)
			on := th.On(th.Background)
			ui.Fill(gtx, th.Background, ui.RadiusLG)
			layout.UniformInset(ui.Space(3)).Layout(gtx, on.Text(ui.TextXS, "Right-anchored panel").Muted().Layout)
			c.resize.Layout(gtx, on)
			return D{Size: size}
		})
		d.Size.X = full
		return d
	}
}

func (g *gallery) sortSection(gtx C, th *ui.Theme) D {
	c := &g.cov
	return ui.Stack(4,
		caption(th, "SortList, vertical (default)"),
		g.sortDemo(th, &c.sortV, &c.idsV, ui.SortList{Gap: ui.Space(1), Within: true}),
		caption(th, "SortList, Horizontal"),
		g.sortDemo(th, &c.sortH, &c.idsH, ui.SortList{Gap: ui.Space(1), Within: true, Horizontal: true}),
		caption(th, "SortList, Snap: the first row is pinned and nothing lands above it"),
		g.sortDemo(th, &c.sortS, &c.idsS, ui.SortList{Gap: ui.Space(1), Within: true, Snap: pinFirst}),
		caption(th, "Skeleton, Width 0 is the whole row"),
		th.Skeleton(0, 14).Layout,
	)(gtx)
}

// pinFirst keeps slot 0 for the pinned row: the pointer over it lands in 1,
// and the pinned row itself stays put.
func pinFirst(active, over int) int {
	if active == 0 {
		return 0
	}
	return max(over, 1)
}

func (g *gallery) sortDemo(th *ui.Theme, s *ui.Sortable, ids *[]string, list ui.SortList) layout.Widget {
	return func(gtx C) D {
		if d, ok := s.Update(gtx); ok && d.ID != d.Over {
			*ids = moveID(*ids, d)
		}
		return s.List(gtx, list, *ids, func(gtx C, i int) D {
			id := (*ids)[i]
			return s.Item(gtx, id, func(gtx C) D { return sortChip(gtx, th, id, s.Dragging() == id) }, nil)
		})
	}
}

// moveID puts the dragged id in the slot of the one it was dropped over; a
// drop on the pinned row leaves the order alone.
func moveID(ids []string, d ui.SortDrop) []string {
	from, to := slices.Index(ids, d.ID), slices.Index(ids, d.Over)
	if from < 0 || to < 0 || ids[to] == coverageSortPinned || ids[from] == coverageSortPinned {
		return ids
	}
	return slices.Insert(slices.Delete(slices.Clone(ids), from, from+1), to, d.ID)
}

func sortChip(gtx C, th *ui.Theme, id string, placeholder bool) D {
	bg := th.Over(ui.Alpha(th.Muted, 0.6))
	label := th.Text(ui.TextSM, id)
	if placeholder {
		bg, label.Color = th.Over(ui.Alpha(th.Muted, 0.2)), th.MutedForeground
	}
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
		func(gtx C) D {
			return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: ui.Space(3), Right: ui.Space(3)}.Layout(gtx, label.Layout)
		},
	)
}

func (g *gallery) labelVariantsSection(gtx C, th *ui.Theme) D {
	long := "A long line of text that wraps over several lines and is cut where MaxLines runs out, with an ellipsis, however much more there is to say"
	italic := th.Text(ui.TextSM, "Italic: the quick brown fox").Italic()
	centered := th.Text(ui.TextSM, "Centered: text-center").Centered()
	two := th.Text(ui.TextSM, long)
	two.MaxLines = 2
	tail := th.Text(ui.TextXS, "~/work/lichdotdev/lich/native/cmd/ui-gallery/sections_coverage.go").Mono().Muted()
	return ui.Stack(4,
		caption(th, "Italic"),
		italic.Layout,
		caption(th, "Centered, laid out at the full width"),
		fullWidth(centered.Layout),
		caption(th, "MaxLines 2"),
		two.Layout,
		caption(th, "LayoutTail, 240dp wide: keeps the end of the path"),
		capped(240, tail.LayoutTail),
	)(gtx)
}

func (g *gallery) iconVariantsSection(gtx C, th *ui.Theme) D {
	glyph := func(ic *icons.Icon, size unit.Dp, c color.NRGBA, filled bool) layout.Widget {
		return func(gtx C) D {
			if filled {
				return ic.LayoutFilled(gtx, size, c)
			}
			return ic.Layout(gtx, size, c)
		}
	}
	var outline, filled, sonner, files []layout.Widget
	for _, name := range []string{"pin", "star", "heart", "bookmark", "bell"} {
		ic := g.icon(name)
		outline = append(outline, glyph(ic, ui.Space(5), th.Foreground, false))
		filled = append(filled, glyph(ic, ui.Space(5), th.Foreground, true))
	}
	tones := map[string]color.NRGBA{"success": th.TonePass, "info": th.Foreground, "warning": th.ToneWait, "error": th.Destructive}
	for _, name := range []string{"success", "info", "warning", "error"} {
		sonner = append(sonner, labeled(th, name, glyph(icons.Sonner(name), ui.Space(5), tones[name], false)))
	}
	for _, name := range []string{"main.go", "package.json", "Dockerfile", "index.html", "style.css", "README.md", ".gitignore", "notes.zzz"} {
		files = append(files, g.deviconFile(th, name))
	}
	mark := icons.LichMark()
	return ui.Stack(4,
		caption(th, "Icon.Layout, then Icon.LayoutFilled"),
		ui.Row(3, outline...),
		ui.Row(3, filled...),
		caption(th, "icons.Sonner"),
		ui.Row(4, sonner...),
		caption(th, "icons.LichMark at 4, 6 and 10"),
		ui.Row(4, glyph(mark, ui.Space(4), th.Foreground, false), glyph(mark, ui.Space(6), th.Foreground, false), glyph(mark, ui.Space(10), th.Foreground, false)),
		caption(th, "icons.DeviconFor, by file name; nil when it has none"),
		func(gtx C) D {
			return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(3)}.Layout(gtx, wrapItems(files)...)
		},
	)(gtx)
}

func wrapItems(ws []layout.Widget) []ui.WrapChild {
	kids := make([]ui.WrapChild, len(ws))
	for i, w := range ws {
		kids[i] = ui.WrapItem(w)
	}
	return kids
}

func (g *gallery) deviconFile(th *ui.Theme, name string) layout.Widget {
	mark := func(gtx C) D {
		b := icons.DeviconFor(name)
		if b == nil {
			return th.Text(ui.TextXS, "nil").Muted().Mono().Layout(gtx)
		}
		return b.Layout(gtx, ui.Space(5), th.Foreground)
	}
	return ui.Row(1.5, mark, caption(th, name))
}

// paletteTokens are every color of the theme's Palette, in its declaration order.
func paletteTokens(p ui.Palette) []struct {
	name  string
	color color.NRGBA
} {
	return []struct {
		name  string
		color color.NRGBA
	}{
		{"Background", p.Background}, {"Foreground", p.Foreground}, {"Canvas", p.Canvas},
		{"Card", p.Card}, {"CardForeground", p.CardForeground}, {"Popover", p.Popover}, {"PopoverForeground", p.PopoverForeground},
		{"Primary", p.Primary}, {"PrimaryForeground", p.PrimaryForeground}, {"Secondary", p.Secondary}, {"SecondaryForeground", p.SecondaryForeground},
		{"Muted", p.Muted}, {"MutedForeground", p.MutedForeground}, {"Accent", p.Accent}, {"AccentForeground", p.AccentForeground},
		{"Destructive", p.Destructive}, {"TonePass", p.TonePass}, {"ToneWait", p.ToneWait},
		{"Border", p.Border}, {"Input", p.Input}, {"Ring", p.Ring},
		{"Sidebar", p.Sidebar}, {"SidebarForeground", p.SidebarForeground}, {"SidebarAccent", p.SidebarAccent},
		{"SidebarAccentForeground", p.SidebarAccentForeground}, {"SidebarBorder", p.SidebarBorder},
	}
}

const paletteSwatch = 5 // Space units: size-5

func (g *gallery) paletteSection(gtx C, th *ui.Theme) D {
	var kids []ui.WrapChild
	for _, t := range paletteTokens(th.Palette) {
		kids = append(kids, ui.WrapItem(ui.Row(2, swatch(th, t.color), th.Text(ui.TextXS, t.name).Mono().Muted().Layout)))
	}
	return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(2)}.Layout(gtx, kids...)
}

// swatch is the token on the card with a hairline around it, so a token as
// dark as the card still reads.
func swatch(th *ui.Theme, c color.NRGBA) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Min = image.Pt(gtx.Dp(ui.Space(paletteSwatch)), gtx.Dp(ui.Space(paletteSwatch)))
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Over(th.Border), ui.RadiusSM) },
			func(gtx C) D {
				return layout.UniformInset(1).Layout(gtx, func(gtx C) D {
					gtx.Constraints.Min = gtx.Constraints.Min.Sub(image.Pt(2, 2))
					return ui.Fill(gtx, th.Over(c), ui.RadiusSM-1)
				})
			},
		)
	}
}
