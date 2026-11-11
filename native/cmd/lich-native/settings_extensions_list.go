package main

import (
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	extensionRowPadY  = ui.Space(2.5)
	extensionIconSize = ui.Space(4)
	extensionEmptyPad = ui.Space(1)
)

// extensionsPaneRows is the pane under its title line: the description, the
// install rows, then the Installed heading over the list or its empty state.
func (s *settingsScreen) extensionsPaneRows(th *ui.Theme) []layout.Widget {
	p := &s.extensions
	rows := []layout.Widget{func(gtx C) D {
		return layout.Inset{Top: -ui.Space(2), Bottom: ui.Space(4)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(settingsDescSize)*settingsDescCh*6/10)
			l := th.Text(settingsDescSize, i18n.T("settings.extensionsSettings.description")).Muted().Leading(20)
			l.MaxLines = 0
			return l.Layout(gtx)
		})
	}}
	rows = append(rows, s.extensionsInstallRows(th)...)
	if p.store == nil {
		return rows
	}
	list := p.store.all()
	if len(list) == 0 {
		return append(rows, groupHeading(th, i18n.T("settings.extensionsSettings.installed"), false), func(gtx C) D {
			return layout.Inset{Left: rowPadX, Top: extensionEmptyPad}.Layout(gtx, func(gtx C) D { return extensionsEmpty(gtx, th) })
		})
	}
	count := i18n.T("settings.extensionsSettings.installedCount", i18n.Args{"n": strconv.Itoa(len(list))})
	rows = append(rows, groupHeading(th, count, false))
	for _, e := range list {
		rows = append(rows, func(gtx C) D { return s.extensionRow(gtx, th, e) })
	}
	return rows
}

// extensionsEmpty is the list's empty state, muted, the size of a row's text.
func extensionsEmpty(gtx C, th *ui.Theme) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.emptyTitle")).Muted().Layout),
		ui.VGap(0.5),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.emptyDescription")).Muted().Wrapped().Layout),
	)
}

// extensionRow is one extension: hover fills it, the right press and the ⋯
// open its menu, and an error from its menu lands on a line under it.
func (s *settingsScreen) extensionRow(gtx C, th *ui.Theme, e backend.Extension) D {
	p := &s.extensions
	r := p.row(e.ID)
	if r.restart.Clicked(gtx) {
		s.restartExtension(e.ID)
	}
	entries := func() []ui.MenuEntry { return s.extensionMenu(e) }
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	row := func(gtx C) D {
		return th.ContextMenuLazy(s.root, &r.context, entries).Layout(gtx, func(gtx C) D {
			return s.extensionRowBody(gtx, th, e, r, entries)
		})
	}
	return withErrorLine(gtx, th, p.snapshot().errorAt(errAtRow(e.ID)), rowPadX, row)
}

// extensionRowBody is the row's line on its hover fill.
func (s *settingsScreen) extensionRowBody(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState, entries func() []ui.MenuEntry) D {
	hovered := r.hover.Update(gtx.Source) || r.menu.Opened() || r.context.Opened()
	bg := th.Surface
	if hovered {
		bg = th.Over(ui.Alpha(th.Accent, hoverFill))
	}
	on := th.On(bg)
	macro := op.Record(gtx.Ops)
	d := layout.Inset{Left: rowPadX, Right: rowPadX, Top: extensionRowPadY, Bottom: extensionRowPadY}.Layout(gtx, func(gtx C) D {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return s.extensions.extensionIcon(e).Layout(gtx, extensionIconSize, on.MutedForeground) }),
			ui.Gap(3),
			layout.Flexed(1, func(gtx C) D { return extensionDetails(gtx, on, e) }),
			ui.Gap(3),
			layout.Rigid(func(gtx C) D { return s.extensionStatus(gtx, on, e, r) }),
			ui.Gap(3),
		}
		if e.Status == api.ExtensionFailed {
			b := on.Button(&r.restart, i18n.T("settings.extensionsSettings.restart"))
			b.Variant, b.Size = ui.VariantOutline, ui.SizeSM
			children = append(children, layout.Rigid(b.Layout), ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return s.extensionMore(gtx, on, e, r, entries()) }))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
	call := macro.Stop()
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	r.hover.Add(gtx.Ops)
	area.Pop()
	if hovered {
		box := gtx
		box.Constraints = layout.Exact(d.Size)
		ui.Fill(box, bg, ui.RadiusMD)
	}
	call.Add(gtx.Ops)
	return d
}

// extensionDetails is the name and version over the source line.
func extensionDetails(gtx C, th *ui.Theme, e backend.Extension) D {
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, e.Name).Weight(font.Medium).Layout)}
	if e.Version != "" {
		name = append(name, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, e.Version).Mono().Muted().Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, name...) }),
		ui.VGap(0.5),
		layout.Rigid(th.Text(ui.TextXS, extensionSourceLine(e)).Mono().Muted().Truncated().Layout),
	)
}

// extensionStatus is the dot and its word, the error one hover away.
func (s *settingsScreen) extensionStatus(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState) D {
	word := th.Text(ui.TextXS, extensionStatusText(e.Status)).In(extensionStatusTone(th, e.Status))
	status := func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return extensionDot(gtx, th, e.Status) }),
			ui.Gap(1.5),
			layout.Rigid(word.Layout),
		)
	}
	if e.Error == "" {
		return status(gtx)
	}
	return th.TooltipCard(s.root, &r.state, func(gtx C) D {
		return th.On(th.Popover).Text(ui.TextXS, e.Error).Mono().Wrapped().Layout(gtx)
	}).Layout(gtx, status)
}

// extensionStatusTone is the status word's color: the destructive tone for
// what failed, the wait tone while restarting.
func extensionStatusTone(th *ui.Theme, status string) color.NRGBA {
	switch status {
	case api.ExtensionFailed, api.ExtensionInvalid:
		return th.Destructive
	case api.ExtensionRestarting:
		return th.ToneWait
	}
	return th.MutedForeground
}

// extensionDot is the status dot: running beats in the pass tone.
func extensionDot(gtx C, th *ui.Theme, status string) D {
	switch status {
	case api.ExtensionRunning:
		return th.BeatingDot(gtx, th.TonePass)
	case api.ExtensionRestarting:
		return ui.StatusDot(gtx, th.ToneWait)
	case api.ExtensionFailed, api.ExtensionInvalid:
		return ui.StatusDot(gtx, th.Destructive)
	}
	return ui.HollowDot(gtx, th.MutedForeground)
}

// extensionMore is the row's ⋯ and the menu it opens.
func (s *settingsScreen) extensionMore(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState, entries []ui.MenuEntry) D {
	hint := i18n.T("settings.extensionsSettings.rowOptions", i18n.Args{"name": e.Name})
	return th.DropdownMenu(s.root, &r.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &r.moreTip, hint).Layout(gtx, func(gtx C) D {
			b := th.Button(&r.more, "")
			b.Icon, b.Variant, b.Size = icExtensionMore, ui.VariantGhost, ui.SizeIconSM
			return b.Layout(gtx)
		})
	})
}

// extensionMenu is a row's menu: Restart for one not running, its folder,
// then Remove…, which asks first.
func (s *settingsScreen) extensionMenu(e backend.Extension) []ui.MenuEntry {
	var entries []ui.MenuEntry
	if e.Status != api.ExtensionRunning {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("settings.extensionsSettings.restart"), Icon: icExtensionRestart, OnClick: func() { s.restartExtension(e.ID) }})
	}
	return append(entries,
		ui.MenuEntry{Label: i18n.T("settings.extensionsSettings.openFolder"), Icon: icExtensionsFolder, OnClick: func() { s.openExtensionDir(e) }},
		ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: i18n.T("settings.extensionsSettings.removeMenuItem"), Icon: icExtensionRemove, Variant: ui.VariantDestructive, OnClick: func() {
			s.extensions.askRemove(e)
			// The dialogs were laid out before the menu took the click.
			s.invalidate()
		}},
	)
}
