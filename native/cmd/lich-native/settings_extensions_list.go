package main

import (
	"image/color"

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
)

// extensionsPaneRows is the pane under its title line: the description, the
// tabs, then the view the selected one names.
func (s *settingsScreen) extensionsPaneRows(th *ui.Theme) []layout.Widget {
	p := &s.extensions
	var list []backend.Extension
	if p.store != nil {
		list = p.store.all()
	}
	rows := []layout.Widget{func(gtx C) D {
		return layout.Inset{Top: -ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, th.Text(settingsDescSize, i18n.T("settings.extensionsSettings.description")).Muted().Leading(20).Wrapped().Layout)
	}, s.extensionsViews(th, list)}
	if p.view.Selected == extensionsViewInstalled {
		return append(rows, s.installedRows(th, list)...)
	}
	if p.catalog != nil {
		rows = append(rows, s.catalogRows(th, list)...)
	}
	return rows
}

// installedRows is a row per extension, or the line saying there is none.
func (s *settingsScreen) installedRows(th *ui.Theme, list []backend.Extension) []layout.Widget {
	if len(list) == 0 {
		return []layout.Widget{func(gtx C) D {
			return layout.Inset{Left: rowPadX}.Layout(gtx, th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.installedEmpty")).Muted().Layout)
		}}
	}
	rows := make([]layout.Widget, len(list))
	for i, e := range list {
		rows[i] = func(gtx C) D { return s.extensionRow(gtx, th, e) }
	}
	return rows
}

// extensionRow is one extension: hover fills it, the right press and the ⋯
// open its menu, and an error from its menu lands on a line under it.
func (s *settingsScreen) extensionRow(gtx C, th *ui.Theme, e backend.Extension) D {
	p := &s.extensions
	r := p.row(e.ID)
	if r.restart.Clicked(gtx) {
		s.restartExtension(e.ID)
	}
	if r.update.Clicked(gtx) {
		s.updateExtension(e)
		// The dialogs were laid out before the row took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	if r.remove.Clicked(gtx) {
		p.askRemove(e)
		gtx.Execute(op.InvalidateCmd{})
	}
	listed := p.listed(e.ID)
	if r.open.Clicked(gtx) && listed {
		p.grid.detail.show(e.ID)
		gtx.Execute(op.InvalidateCmd{})
	}
	entries := func() []ui.MenuEntry { return s.extensionMenu(e) }
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	row := func(gtx C) D {
		return th.ContextMenuLazy(s.root, &r.context, entries).Layout(gtx, func(gtx C) D {
			return s.extensionRowBody(gtx, th, e, r, entries, listed)
		})
	}
	return withErrorLine(gtx, th, p.snapshot().errorAt(errAtRow(e.ID)), rowPadX, row)
}

// extensionRowBody is the row's line on its hover fill. A press on the row
// outside its buttons opens the catalog's detail while the catalog lists it.
func (s *settingsScreen) extensionRowBody(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState, entries func() []ui.MenuEntry, listed bool) D {
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
		switch {
		case e.Revoked != "":
			b := on.Button(&r.remove, i18n.T("settings.extensionsSettings.removeMenuItem"))
			b.Variant, b.Size = ui.VariantOutline, ui.SizeSM
			children = append(children, layout.Rigid(b.Layout), ui.Gap(1))
		case e.Status == api.ExtensionFailed:
			b := on.Button(&r.restart, i18n.T("settings.extensionsSettings.restart"))
			b.Variant, b.Size = ui.VariantOutline, ui.SizeSM
			children = append(children, layout.Rigid(b.Layout), ui.Gap(1))
		}
		if e.Latest != "" {
			children = append(children, layout.Rigid(s.extensions.updateButton(on, e, r)), ui.Gap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D { return s.extensionMore(gtx, on, e, r, entries()) }))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
	call := macro.Stop()
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	r.hover.Add(gtx.Ops)
	area.Pop()
	box := gtx
	box.Constraints = layout.Exact(d.Size)
	if hovered {
		ui.Fill(box, bg, ui.RadiusMD)
	}
	if listed {
		r.open.Layout(box, func(gtx C) D { return D{Size: gtx.Constraints.Min} })
	}
	call.Add(gtx.Ops)
	return d
}

// listed is whether the catalog lists id, whose installed row then opens the
// same detail as its card.
func (p *extensionsSettings) listed(id string) bool {
	if p.catalog == nil {
		return false
	}
	cat, _ := p.catalog.read()
	if cat == nil {
		return false
	}
	_, ok := detailEntry(cat, id)
	return ok
}

// updateButton is Update to the newer release, busy while this row's update
// runs and off while any install does.
func (p *extensionsSettings) updateButton(th *ui.Theme, e backend.Extension, r *extensionRowState) layout.Widget {
	b := th.Button(&r.update, i18n.T("settings.extensionsSettings.updateTo", i18n.Args{"version": e.Latest}))
	installing := p.snapshot().installing
	if installing == errAtRow(e.ID) {
		b.Text = i18n.T("settings.extensionsSettings.updating")
	}
	b.Variant, b.Size, b.Disabled = ui.VariantOutline, ui.SizeSM, installing != ""
	return b.Layout
}

// extensionDetails is the name and version over the second line.
func extensionDetails(gtx C, th *ui.Theme, e backend.Extension) D {
	name := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, extensionName(e)).Weight(font.Medium).Layout)}
	if e.Version != "" {
		name = append(name, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, e.Version).Mono().Muted().Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, name...) }),
		ui.VGap(0.5),
		layout.Rigid(extensionSecondLine(th, e).Truncated().Layout),
	)
}

// extensionSecondLine is why the catalog took a revoked extension down, else
// its description, else where it came from: the manifest asks a description
// only of a listed one.
func extensionSecondLine(th *ui.Theme, e backend.Extension) ui.LabelStyle {
	if e.Revoked != "" {
		return th.Text(ui.TextXS, i18n.T("settings.extensionsSettings.removedFromCatalog", i18n.Args{"reason": e.Revoked})).In(th.Destructive)
	}
	if desc := extensionDescription(e); desc != "" {
		return th.Text(ui.TextXS, desc).Muted()
	}
	return th.Text(ui.TextXS, extensionSourceLine(e)).Mono().Muted()
}

// rowStatus is the status a row shows: a revoked extension is stopped for
// good, its reason already on the row's second line.
func rowStatus(e backend.Extension) string {
	if e.Revoked != "" {
		return api.ExtensionStopped
	}
	return e.Status
}

// extensionStatus is the dot and its word, the source, the error and the
// permissions one hover away.
func (s *settingsScreen) extensionStatus(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState) D {
	status := rowStatus(e)
	word := th.Text(ui.TextXS, extensionStatusText(status)).In(extensionStatusTone(th, status))
	line := func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return extensionDot(gtx, th, status) }),
			ui.Gap(1.5),
			layout.Rigid(word.Layout),
		)
	}
	if extensionSourceLine(e) == "" && tipError(e) == "" && len(e.Permissions) == 0 {
		return line(gtx)
	}
	return th.TooltipCard(s.root, &r.state, func(gtx C) D { return statusTip(gtx, th.On(th.Popover), e) }).Layout(gtx, line)
}

// tipError is the error the tooltip shows: none for a revoked extension,
// whose row already says why.
func tipError(e backend.Extension) string {
	if e.Revoked != "" {
		return ""
	}
	return e.Error
}

// statusTip is the status's tooltip: where the extension came from, the
// error, then what it can do, in the confirmation's words.
func statusTip(gtx C, th *ui.Theme, e backend.Extension) D {
	var children []layout.FlexChild
	add := func(w layout.Widget) {
		if len(children) > 0 {
			children = append(children, ui.VGap(3))
		}
		children = append(children, layout.Rigid(w))
	}
	if src := extensionSourceLine(e); src != "" {
		add(th.Text(ui.TextXS, src).Mono().Muted().Wrapped().Layout)
	}
	if err := tipError(e); err != "" {
		add(th.Text(ui.TextXS, err).Mono().Wrapped().Layout)
	}
	if len(e.Permissions) > 0 {
		add(func(gtx C) D { return permissionList(gtx, th, e.Permissions, nil) })
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
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
	hint := i18n.T("settings.extensionsSettings.rowOptions", i18n.Args{"name": extensionName(e)})
	return th.DropdownMenu(s.root, &r.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &r.moreTip, hint).Layout(gtx, func(gtx C) D {
			b := th.Button(&r.more, "")
			b.Icon, b.Variant, b.Size = icExtensionMore, ui.VariantGhost, ui.SizeIconSM
			return b.Layout(gtx)
		})
	})
}

// extensionMenu is a row's menu: Restart for one not running and not
// revoked, its folder, then Remove…, which asks first.
func (s *settingsScreen) extensionMenu(e backend.Extension) []ui.MenuEntry {
	var entries []ui.MenuEntry
	if e.Status != api.ExtensionRunning && e.Revoked == "" {
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
