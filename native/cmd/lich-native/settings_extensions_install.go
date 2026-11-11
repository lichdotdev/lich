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
)

// extensionsTitle is the section's title line, Open folder on its right, and
// the line under it when Open folder failed.
func (s *settingsScreen) extensionsTitle(th *ui.Theme, label string) layout.Widget {
	return func(gtx C) D {
		p := &s.extensions
		if p.folder.Clicked(gtx) {
			go s.openExtensionsFolder()
		}
		open := th.Button(&p.folder, i18n.T("settings.extensionsSettings.openFolder"))
		open.Variant, open.Size, open.Icon = ui.VariantGhost, ui.SizeSM, icExtensionsFolder
		line := func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, th.Text(settingsTitle, label).Weight(font.SemiBold).Leading(32).Layout),
				layout.Rigid(open.Layout),
			)
		}
		return layout.Inset{Bottom: extensionTitleBelow}.Layout(gtx, func(gtx C) D {
			return withErrorLine(gtx, th, p.snapshot().errorAt(errAtFolder), 0, line)
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

// extensionsInstallRows are the two install rows: from GitHub, from a folder.
func (s *settingsScreen) extensionsInstallRows(th *ui.Theme) []layout.Widget {
	p := &s.extensions
	git := settingRow{title: i18n.T("settings.extensionsSettings.fromGit"), description: i18n.T("settings.extensionsSettings.gitDescription")}
	dir := settingRow{title: i18n.T("settings.extensionsSettings.fromDir"), description: i18n.T("settings.extensionsSettings.dirDescription")}
	return []layout.Widget{
		func(gtx C) D {
			if p.urlEvents(gtx) || p.install.Clicked(gtx) {
				p.askInstall(installFromGit, p.url.Text())
				// The dialogs were laid out before the row took the click.
				gtx.Execute(op.InvalidateCmd{})
			}
			t := p.snapshot()
			return withErrorLine(gtx, th, t.errorAt(installFromGit), rowPadX, func(gtx C) D {
				return git.layout(gtx, th, true, func(gtx C, th *ui.Theme) D { return p.gitControl(gtx, th, t.installing) })
			})
		},
		func(gtx C) D {
			t := p.snapshot()
			if p.choose.Clicked(gtx) && t.installing == "" {
				s.pickExtensionDir()
			}
			return withErrorLine(gtx, th, t.errorAt(installFromDir), rowPadX, func(gtx C) D {
				return dir.layout(gtx, th, false, func(gtx C, th *ui.Theme) D { return p.dirControl(gtx, th, t.installing) })
			})
		},
	}
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

// layoutExtensionDialogs draws the install and remove confirmations. The
// window lays it out with its own constraints, before the content, as every
// dialog is.
func (s *settingsScreen) layoutExtensionDialogs(gtx C, th *ui.Theme) {
	p := &s.extensions
	t := p.snapshot()
	asking := t.asking != ""
	install := extensionConfirm{state: &p.installDialog, slot: 0, open: asking,
		title: i18n.T("settings.extensionsSettings.installTitle"), action: i18n.T("settings.extensionsSettings.install"),
		desc: i18n.T("settings.extensionsSettings.installDescription", i18n.Args{"source": t.asking})}
	if pressed, closed := p.confirmDialog(gtx, th, install); pressed {
		s.installExtension(t.askingFrom == installFromDir, t.asking)
	} else if closed {
		p.with(func(t *extensionTasks) { t.asking = "" })
	}
	remove := extensionConfirm{state: &p.removeDialog, slot: 1, open: t.removal != nil, destructive: true,
		title: i18n.T("settings.extensionsSettings.removeTitle"), action: i18n.T("settings.extensionsSettings.remove")}
	if t.removal != nil {
		remove.desc = i18n.T("settings.extensionsSettings.removeDescription", i18n.Args{"name": t.removal.Name})
	}
	if pressed, closed := p.confirmDialog(gtx, th, remove); pressed {
		s.removeExtension(*t.removal)
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
	dlg.Actions = []layout.Widget{cancel.Layout, confirm.Layout}
	if dlg.Layout(gtx) {
		closed = true
	}
	return pressed, closed && !pressed
}
