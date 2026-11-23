package main

import (
	"image"
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

// extensionFootH is a card's foot, as tall as its buttons whether it has
// any or not.
var extensionFootH = ui.Space(6)

// extensionsPaneRows is the pane under its title line: the tabs, then the view
// the selected one names.
func (s *settingsScreen) extensionsPaneRows(th *ui.Theme) []layout.Widget {
	p := &s.extensions
	var list []backend.Extension
	if p.store != nil {
		list = p.store.all()
	}
	rows := []layout.Widget{s.extensionsViews(th, list)}
	if p.view.Selected == extensionsViewInstalled {
		return append(rows, s.installedRows(th, list)...)
	}
	if p.catalog != nil {
		rows = append(rows, s.catalogRows(th, list)...)
	}
	return rows
}

// installedRows is the installed extensions as cards of the catalog's grid,
// or the line saying there is none.
func (s *settingsScreen) installedRows(th *ui.Theme, list []backend.Extension) []layout.Widget {
	if len(list) == 0 {
		return []layout.Widget{func(gtx C) D {
			return layout.Inset{Left: rowPadX}.Layout(gtx, th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.installedEmpty")).Muted().Layout)
		}}
	}
	return []layout.Widget{func(gtx C) D {
		return catalogCards(gtx, len(list), func(gtx C, i int) D { return s.extensionCard(gtx, th, list[i]) })
	}}
}

// extensionCard is one installed extension as the catalog draws a listing:
// its cover, its name and version with the ⋯, two lines of description, and
// its status with what fixes it at the foot. A click on it opens the
// catalog's detail while the catalog lists it; the right press and the ⋯
// open its menu, and an error from its menu lands on a line under it.
func (s *settingsScreen) extensionCard(gtx C, th *ui.Theme, e backend.Extension) D {
	p := &s.extensions
	r := p.row(e.ID)
	if r.restart.Clicked(gtx) {
		s.restartExtension(e.ID)
	}
	if r.update.Clicked(gtx) {
		s.updateExtension(e)
		// The dialogs were laid out before the card took the click.
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
	card := func(gtx C) D {
		return th.ContextMenuLazy(s.root, &r.context, entries).Layout(gtx, func(gtx C) D {
			return s.extensionCardBody(gtx, th, e, r, entries, listed)
		})
	}
	return withErrorLine(gtx, th, p.snapshot().errorAt(errAtRow(e.ID)), catalogTextX, card)
}

// extensionCardBody is the card on its fill. Its press sits under the card's
// buttons rather than around them, so a press on the ⋯ or Update is theirs
// alone.
func (s *settingsScreen) extensionCardBody(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState, entries func() []ui.MenuEntry, listed bool) D {
	hovered := r.hover.Update(gtx.Source) || r.menu.Opened() || r.context.Opened()
	bg := th.Card
	if hovered {
		bg = ui.MixOKLab(th.Card, th.Over(th.Accent), catalogHoverMix)
	}
	on := th.On(bg)
	macro := op.Record(gtx.Ops)
	d := catalogCardSurface(gtx, bg, func(gtx C) D { return s.extensionCover(gtx, on, e) }, func(gtx C) D {
		return s.extensionCardText(gtx, on, e, r, entries)
	})
	call := macro.Stop()
	area := clip.Rect{Max: d.Size}.Push(gtx.Ops)
	r.hover.Add(gtx.Ops)
	area.Pop()
	if listed {
		box := gtx
		box.Constraints = layout.Exact(d.Size)
		r.open.Layout(box, func(gtx C) D { return D{Size: gtx.Constraints.Min} })
	}
	call.Add(gtx.Ops)
	return d
}

// extensionCover is the listing's cover while the catalog lists the
// extension with one, else its icon on the empty cover's ground.
func (s *settingsScreen) extensionCover(gtx C, th *ui.Theme, e backend.Extension) D {
	if s.extensions.catalog != nil {
		if cat, _ := s.extensions.catalog.read(); cat != nil {
			if entry, ok := detailEntry(cat, e.ID); ok && entry.Cover != "" {
				return s.catalogCover(gtx, th, entry)
			}
		}
	}
	w := gtx.Constraints.Max.X
	gtx.Constraints = layout.Exact(image.Pt(w, w*catalogCoverH/catalogCoverW))
	ui.Fill(gtx, ui.MixOKLab(th.Surface, th.Background, catalogCoverMix), 0)
	return layout.Center.Layout(gtx, func(gtx C) D {
		return s.extensions.extensionIcon(e).Layout(gtx, catalogGlyphSize, th.MutedForeground)
	})
}

// extensionCardText is the card under its cover. The description always
// holds its two lines and the foot its button's height, so every card in the
// grid stands as tall as the next.
func (s *settingsScreen) extensionCardText(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState, entries func() []ui.MenuEntry) D {
	name := []layout.FlexChild{layout.Flexed(1, func(gtx C) D {
		line := []layout.FlexChild{layout.Rigid(th.Text(catalogNameSize, extensionName(e)).Weight(font.SemiBold).Leading(catalogNameLeading).Truncated().Layout)}
		if e.Version != "" {
			line = append(line, ui.Gap(2), layout.Rigid(th.Text(ui.TextXS, e.Version).Mono().Muted().Layout))
		}
		return layout.Flex{Alignment: layout.Baseline}.Layout(gtx, line...)
	})}
	name = append(name, layout.Rigid(func(gtx C) D { return s.extensionMore(gtx, th, e, r, entries()) }))
	desc := extensionSecondLine(th, e).Leading(catalogDescLeading)
	desc.MaxLines = catalogDescLines
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return layout.Flex{Alignment: layout.Middle}.Layout(gtx, name...) }),
		ui.VGap(1),
		layout.Rigid(func(gtx C) D {
			d := desc.Layout(gtx)
			d.Size.Y = max(d.Size.Y, catalogDescLines*gtx.Sp(catalogDescLeading))
			return d
		}),
		layout.Rigid(layout.Spacer{Height: catalogMetaAbove}.Layout),
		layout.Rigid(func(gtx C) D { return s.extensionCardFoot(gtx, th, e, r) }),
	)
}

// extensionCardFoot is the status on the left and, on the right, Remove for
// a revoked extension, Restart for a failed one and Update while a newer
// release waits.
func (s *settingsScreen) extensionCardFoot(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return s.extensionStatus(gtx, th, e, r) }),
		layout.Flexed(1, func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Min.X, 0)} }),
	}
	button := func(c *ui.Clickable, key string) {
		children = append(children, ui.Gap(1), layout.Rigid(func(gtx C) D {
			b := th.Button(c, i18n.T(key))
			b.Variant, b.Size = ui.VariantOutline, ui.SizeXS
			return b.Layout(gtx)
		}))
	}
	switch {
	case e.Revoked != "":
		button(&r.remove, "settings.extensionsSettings.removeMenuItem")
	case e.Status == api.ExtensionFailed:
		button(&r.restart, "settings.extensionsSettings.restart")
	}
	if e.Latest != "" {
		children = append(children, ui.Gap(1), layout.Rigid(s.extensions.updateButton(th, e, r)))
	}
	gtx.Constraints.Min.Y = gtx.Dp(extensionFootH)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
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
	b.Variant, b.Size, b.Disabled = ui.VariantOutline, ui.SizeXS, installing != ""
	return b.Layout
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

// extensionMore is the card's ⋯ and the menu it opens.
func (s *settingsScreen) extensionMore(gtx C, th *ui.Theme, e backend.Extension, r *extensionRowState, entries []ui.MenuEntry) D {
	hint := i18n.T("settings.extensionsSettings.rowOptions", i18n.Args{"name": extensionName(e)})
	return th.DropdownMenu(s.root, &r.menu, entries).Layout(gtx, func(gtx C) D {
		return th.Tooltip(s.root, &r.moreTip, hint).Layout(gtx, func(gtx C) D {
			b := th.Button(&r.more, "")
			b.Icon, b.Variant, b.Size = icExtensionMore, ui.VariantGhost, ui.SizeIconXS
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
