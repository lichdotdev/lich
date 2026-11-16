package main

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	extensionURLFieldW  = ui.Space(72)
	extensionFieldH     = ui.Space(8)
	extensionErrorPad   = ui.Space(3)
	extensionTitleBelow = ui.Space(4)
	// The Install from… panel, from the approved mockup (extensions-screen,
	// frame 3): 400px wide, 12px around its two sources.
	extensionInstallPanelW   = ui.Space(100)
	extensionInstallPanelPad = ui.Space(2)
	extensionSourceGap       = ui.Space(3)
)

// extensionsTitle is the section's title line, Install from… on its right.
// The panel gives way to the confirmation once a source is staged.
func (s *settingsScreen) extensionsTitle(th *ui.Theme, label string) layout.Widget {
	return func(gtx C) D {
		p := &s.extensions
		if p.snapshot().asking {
			p.installPanel.Close()
		}
		pop := th.DropdownPanel(s.root, &p.installPanel, extensionInstallPanelW, func(gtx C, th *ui.Theme) D {
			return layout.UniformInset(extensionInstallPanelPad).Layout(gtx, func(gtx C) D { return s.installSources(gtx, th) })
		})
		pop.AlignEnd = true
		open := th.Button(&p.installFrom, i18n.T("settings.extensionsSettings.installFrom"))
		open.Variant, open.Size, open.Icon, open.Lit = ui.VariantOutline, ui.SizeSM, icExtensionAdd, p.installPanel.Opened()
		return layout.Inset{Bottom: extensionTitleBelow}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, th.Text(settingsTitle, label).Weight(font.SemiBold).Leading(32).Layout),
				layout.Rigid(func(gtx C) D { return pop.Layout(gtx, open.Layout) }),
			)
		})
	}
}

// withErrorLine is w over text on one line in the destructive tone, at the
// row's left edge padX; text "" is w alone.
func withErrorLine(gtx C, th *ui.Theme, text string, padX unit.Dp, w layout.Widget) D {
	if text == "" {
		return w(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(w),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: padX, Right: padX, Bottom: extensionErrorPad}.Layout(gtx, th.Text(ui.TextXS, text).In(th.Destructive).Layout)
		}),
	)
}

// installSources is the Install from… panel: from GitHub over from a folder,
// each with why it failed under it.
func (s *settingsScreen) installSources(gtx C, th *ui.Theme) D {
	p := &s.extensions
	if p.urlEvents(gtx) || p.install.Clicked(gtx) {
		s.prepareInstall(installFromGit, p.url.Text())
		// The dialogs were laid out before the panel took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	t := p.snapshot()
	if p.choose.Clicked(gtx) && t.installing == "" {
		s.pickExtensionDir()
	}
	git := installSource{title: i18n.T("settings.extensionsSettings.fromGit"), description: i18n.T("settings.extensionsSettings.gitDescription"),
		err: t.errorAt(installFromGit), control: func(gtx C) D { return p.gitControl(gtx, th, t.installing) }}
	dir := installSource{title: i18n.T("settings.extensionsSettings.fromDir"), description: i18n.T("settings.extensionsSettings.dirDescription"),
		err: t.errorAt(installFromDir), control: func(gtx C) D { return p.dirControl(gtx, th, t.installing) }}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return git.layout(gtx, th) }),
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: extensionSourceGap, Bottom: extensionSourceGap}.Layout(gtx, th.Separator)
		}),
		layout.Rigid(func(gtx C) D { return dir.layout(gtx, th) }),
	)
}

// installSource is one way in of the Install from… panel.
type installSource struct {
	title, description, err string
	control                 layout.Widget
}

func (r installSource) layout(gtx C, th *ui.Theme) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextSM, r.title).Weight(font.Medium).Layout),
		ui.VGap(0.5),
		layout.Rigid(th.Text(ui.TextXS, r.description).Muted().Wrapped().Layout),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			// The panel hands its width down as a minimum; a button keeps its own.
			gtx.Constraints.Min.X = 0
			return r.control(gtx)
		}),
		layout.Rigid(func(gtx C) D {
			if r.err == "" {
				return D{}
			}
			return layout.Inset{Top: ui.Space(2)}.Layout(gtx, th.Text(ui.TextXS, r.err).In(th.Destructive).Wrapped().Layout)
		}),
	)
}

// urlEvents reports Enter in the URL field, and drops the GitHub row's error
// once the URL changes.
func (p *extensionsSettings) urlEvents(gtx C) bool {
	p.url.Submit = true
	hit := false
	for {
		ev, more := p.url.Update(gtx)
		if !more {
			return hit
		}
		switch ev.(type) {
		case widget.SubmitEvent:
			hit = true
		case widget.ChangeEvent:
			p.clearError(installFromGit)
		}
	}
}

// gitControl is the URL field and Install, which shows the busy state while
// its install runs.
func (p *extensionsSettings) gitControl(gtx C, th *ui.Theme, installing string) D {
	install := th.Button(&p.install, i18n.T("settings.extensionsSettings.install"))
	if installing == installFromGit {
		install.Text = i18n.T("settings.extensionsSettings.installing")
	}
	install.Size, install.Disabled = ui.SizeSM, installing != "" || p.url.Text() == ""
	field := th.Input(&p.url, extensionPlaceholder)
	field.Mono, field.TextSize, field.Disabled, field.Height = true, ui.TextXS, installing != "", extensionFieldH
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			w := min(gtx.Constraints.Max.X, gtx.Dp(extensionURLFieldW))
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return field.Layout(gtx)
		}),
		ui.Gap(2),
		layout.Rigid(install.Layout),
	)
}

// dirControl is Choose folder…, which shows the busy state while its install
// runs.
func (p *extensionsSettings) dirControl(gtx C, th *ui.Theme, installing string) D {
	choose := th.Button(&p.choose, i18n.T("settings.extensionsSettings.chooseFolder"))
	if installing == installFromDir {
		choose.Text = i18n.T("settings.extensionsSettings.installing")
	}
	choose.Variant, choose.Size, choose.Icon, choose.Disabled = ui.VariantOutline, ui.SizeSM, icExtensionPick, installing != ""
	return choose.Layout(gtx)
}

// layoutExtensionDialogs draws the catalog's detail and the install, update
// and remove confirmations. The
// window lays it out with its own constraints, before the content, as every
// dialog is.
func (s *settingsScreen) layoutExtensionDialogs(gtx C, th *ui.Theme) {
	p := &s.extensions
	t := p.snapshot()
	if t.asking {
		p.grid.detail.id = ""
	}
	s.layoutCatalogDetail(gtx, th, t)
	install := extensionConfirm{state: &p.installDialog, slot: 0, open: t.asking,
		title: installTitle(t.preview), action: i18n.T("settings.extensionsSettings.install"),
		desc: previewSource(t.preview),
		body: func(gtx C) D { return previewWarning(gtx, th.On(th.Popover), t.preview, nil) }}
	if e := t.replacing; e != nil {
		added := newPermissions(e.Permissions, t.preview.Permissions)
		install.title = i18n.T("settings.extensionsSettings.updateTitle", i18n.Args{"name": extensionName(*e), "version": t.preview.Release})
		install.action = i18n.T("settings.extensionsSettings.update")
		install.body = func(gtx C) D { return previewWarning(gtx, th.On(th.Popover), t.preview, added) }
	}
	if pressed, closed := p.confirmDialog(gtx, th, install); pressed && t.replacing != nil {
		go s.commitUpdate(*t.replacing, t.preview)
	} else if pressed {
		s.commitInstall()
	} else if closed && t.asking {
		s.discardInstall()
	}
	remove := extensionConfirm{state: &p.removeDialog, slot: 1, open: t.removal != nil, destructive: true,
		action: i18n.T("settings.extensionsSettings.remove"),
		body:   func(gtx C) D { return p.deleteDataRow(gtx, th.On(th.Popover)) }}
	if t.removal != nil {
		remove.title = i18n.T("settings.extensionsSettings.removeTitle", i18n.Args{"name": extensionName(*t.removal)})
		remove.desc = i18n.T("settings.extensionsSettings.removeDescription")
	}
	if pressed, closed := p.confirmDialog(gtx, th, remove); pressed {
		s.confirmRemoval(*t.removal)
	} else if closed {
		p.with(func(t *extensionTasks) { t.removal = nil })
	}
}

// extensionConfirm is one of the section's confirmations; slot picks its
// buttons' state.
type extensionConfirm struct {
	state               *ui.DialogState
	slot                int
	open, destructive   bool
	title, desc, action string
	body                layout.Widget
}

// deleteDataRow is the remove confirmation's box with its label, which a
// click also ticks.
func (p *extensionsSettings) deleteDataRow(gtx C, th *ui.Theme) D {
	if p.deleteDataLabel.Clicked(gtx) {
		p.deleteData.Value = !p.deleteData.Value
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Checkbox(&p.deleteData).Layout),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			return p.deleteDataLabel.Layout(gtx, th.Text(ui.TextSM, i18n.T("settings.extensionsSettings.removeDeleteData")).Layout)
		}),
	)
}

// confirmDialog is Cancel and the action at sm:max-w-xl. It reports whether
// the action was pressed, and whether the dialog was closed without it.
func (p *extensionsSettings) confirmDialog(gtx C, th *ui.Theme, c extensionConfirm) (pressed, closed bool) {
	if !c.open && !c.state.Animating(gtx) {
		return false, false
	}
	pressed = p.confirm[c.slot].Clicked(gtx) && c.open
	closed = p.cancel[c.slot].Clicked(gtx)
	on := th.On(th.Popover)
	cancel := on.Button(&p.cancel[c.slot], i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	confirm := on.Button(&p.confirm[c.slot], c.action)
	if c.destructive {
		confirm.Variant = ui.VariantDestructive
	}
	dlg := th.Dialog(c.state, c.open, c.title, c.desc)
	dlg.MaxWidth = ui.DialogWidthXL
	dlg.Body = c.body
	dlg.Actions = []layout.Widget{cancel.Layout, confirm.Layout}
	if dlg.Layout(gtx) {
		closed = true
	}
	return pressed, closed && !pressed
}
