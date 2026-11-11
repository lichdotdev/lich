package main

import (
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// extensionsInstallBox is the toggle between GitHub and a folder, the field
// and Install for the one in view, the line saying what lich does with it,
// and the last error.
func (s *settingsScreen) extensionsInstallBox(gtx C, th *ui.Theme) D {
	p := &s.extensions
	t := p.snapshot()
	// The toggle takes its click as it is drawn, after the hint and the field
	// read which source is in view.
	if p.from.Update(gtx) {
		p.with(func(t *extensionTasks) { t.err = "" })
		t.err = ""
	}
	if p.choose.Clicked(gtx) && !t.installing {
		s.pickExtensionDir()
	}
	if p.install.Clicked(gtx) || p.submitted(gtx) {
		p.askInstall()
		// The dialogs were laid out before the box took the click.
		gtx.Execute(op.InvalidateCmd{})
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Sidebar, ui.RadiusMD) },
		func(gtx C) D {
			on := th.On(th.Sidebar)
			return layout.UniformInset(ui.Space(3)).Layout(gtx, func(gtx C) D { return p.installBody(gtx, on, t) })
		},
	)
}

// submitted reports Enter in the field in view.
func (p *extensionsSettings) submitted(gtx C) bool {
	field := &p.url
	if p.from.Value == installFromDir {
		field = &p.dir
	}
	field.Submit = true
	hit := false
	for {
		ev, more := field.Update(gtx)
		if !more {
			return hit
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			hit = true
		}
	}
}

func (p *extensionsSettings) installBody(gtx C, th *ui.Theme, t extensionTasks) D {
	hint := i18n.T("settings.extensionsSettings.gitHint")
	if p.from.Value == installFromDir {
		hint = i18n.T("settings.extensionsSettings.dirHint")
	}
	children := []layout.FlexChild{
		layout.Rigid(th.ToggleGroup(&p.from,
			ui.ToggleItem{Value: installFromGit, Text: i18n.T("settings.extensionsSettings.fromGit"), Disabled: t.installing},
			ui.ToggleItem{Value: installFromDir, Text: i18n.T("settings.extensionsSettings.fromDir"), Disabled: t.installing},
		).Layout),
		ui.VGap(3),
		layout.Rigid(func(gtx C) D { return p.installField(gtx, th, t.installing) }),
		ui.VGap(2),
		layout.Rigid(th.Text(ui.TextXS, hint).Muted().Wrapped().Layout),
	}
	if t.err != "" {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D {
			l := th.Text(ui.TextXS, t.err)
			l.Color = th.Destructive
			return l.Wrapped().Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// installField is the URL field, or the folder field with Choose folder…,
// then Install, which shows the busy state while an install runs.
func (p *extensionsSettings) installField(gtx C, th *ui.Theme, busy bool) D {
	install := th.Button(&p.install, i18n.T("settings.extensionsSettings.install"))
	if busy {
		install.Text = i18n.T("settings.extensionsSettings.installing")
	}
	install.Size, install.Disabled = ui.SizeSM, busy || p.source() == ""
	children := []layout.FlexChild{}
	if p.from.Value == installFromDir {
		field := th.Input(&p.dir, i18n.T("settings.extensionsSettings.dirPlaceholder"))
		field.Mono, field.TextSize, field.Disabled, field.Height = true, ui.TextXS, busy, ui.Space(8)
		choose := th.Button(&p.choose, i18n.T("settings.extensionsSettings.chooseFolder"))
		choose.Variant, choose.Size, choose.Icon, choose.Disabled = ui.VariantOutline, ui.SizeSM, icExtensionPick, busy
		children = append(children, layout.Flexed(1, field.Layout), ui.Gap(2), layout.Rigid(choose.Layout))
	} else {
		field := th.Input(&p.url, extensionPlaceholder)
		field.Mono, field.TextSize, field.Disabled, field.Height = true, ui.TextXS, busy, ui.Space(8)
		children = append(children, layout.Flexed(1, field.Layout))
	}
	children = append(children, ui.Gap(2), layout.Rigid(install.Layout))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
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
		s.installExtension(p.from.Value == installFromDir, t.asking)
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
