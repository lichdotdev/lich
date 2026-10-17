package main

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// importPlaceholder is the repository field's example, a URL and no prose.
const importPlaceholder = "https://github.com/you/lich-themes.git"

// repositoryManifest is the file a theme repository holds, named in the
// import dialog's hint.
const repositoryManifest = "lich-theme.json"

// importDialogWidth is ImportThemeDialog's sm:max-w-lg.
const importDialogWidth unit.Dp = 512

var (
	icFileJSON = icons.Lucide("file-braces") // lucide-react FileJson
	icDownload = icons.Lucide("download")
)

// themeDialogs is the widget state of the import dialog and the three
// confirmations.
type themeDialogs struct {
	imp                              ui.DialogState
	url                              widget.Editor
	install, choose, template, close widget.Clickable

	remove, overwrite, pack ui.DialogState
	cancel, confirm         [3]widget.Clickable
}

// layoutDialogs draws the picker's dialogs. The window lays it out with
// its own constraints, before the content, as every dialog is.
func (p *themePicker) layoutDialogs(gtx C, th *ui.Theme) {
	t := p.snapshot()
	p.importDialog(gtx, th, t)
	d := &p.dialogs
	dismiss := func(change func(t *themeTasks)) func() { return func() { p.with(change) } }
	removal := confirmSpec{state: &d.remove, slot: 0, open: t.removal != nil, title: i18n.T("settings.appearanceSettings.removeThemeTitle"), action: i18n.T("settings.appearanceSettings.removeTheme"),
		closed: dismiss(func(t *themeTasks) { t.removal = nil })}
	if t.removal != nil {
		removal.desc = i18n.T("settings.appearanceSettings.removeThemeDescription", i18n.Args{"name": t.removal.name})
	}
	if p.confirmDialog(gtx, th, removal) {
		p.removeTheme(*t.removal)
	}
	overwrite := confirmSpec{state: &d.overwrite, slot: 1, open: t.overwrite != nil, title: i18n.T("settings.appearanceSettings.replaceThemeTitle"), action: i18n.T("settings.appearanceSettings.replaceTheme"),
		closed: dismiss(func(t *themeTasks) { t.overwrite = nil })}
	if t.overwrite != nil {
		overwrite.desc = i18n.T("settings.appearanceSettings.replaceThemeDescription", i18n.Args{"id": t.overwrite.id})
	}
	if p.confirmDialog(gtx, th, overwrite) {
		p.overwriteFile(*t.overwrite)
	}
	pack := confirmSpec{state: &d.pack, slot: 2, open: t.pack != nil, busy: t.importing, title: i18n.T("settings.appearanceSettings.replaceThemesTitle"),
		desc: i18n.T("settings.appearanceSettings.replaceThemesDescription", i18n.Args{"themes": joinConflicts(t.pack)}), action: i18n.T("settings.appearanceSettings.replaceThemes"),
		closed: dismiss(func(t *themeTasks) { t.pack = nil })}
	if p.confirmDialog(gtx, th, pack) {
		p.installRepository(t.pack.url, true)
	}
}

// confirmSpec is one ConfirmDialog: slot picks its buttons' state, busy
// disables its action, closed runs on Cancel and on any dismissal.
type confirmSpec struct {
	state               *ui.DialogState
	slot                int
	open, busy          bool
	title, desc, action string
	closed              func()
}

// confirmDialog is ConfirmDialog: Cancel and one destructive action at
// sm:max-w-xl. It reports whether the action was pressed.
func (p *themePicker) confirmDialog(gtx C, th *ui.Theme, c confirmSpec) bool {
	d := &p.dialogs
	pressed := d.confirm[c.slot].Clicked(gtx) && c.open && !c.busy
	if d.cancel[c.slot].Clicked(gtx) {
		c.closed()
	}
	on := th.On(th.Popover)
	cancel := on.Button(&d.cancel[c.slot], i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	confirm := on.Button(&d.confirm[c.slot], c.action)
	confirm.Variant, confirm.Disabled = ui.VariantDestructive, c.busy
	dlg := th.Dialog(c.state, c.open, c.title, c.desc)
	dlg.MaxWidth = ui.DialogWidthXL
	dlg.Actions = []layout.Widget{cancel.Layout, confirm.Layout}
	if dlg.Layout(gtx) {
		c.closed()
	}
	return pressed
}

// importDialog is ImportThemeDialog: a repository to clone, or a single
// file through the backend's picker, and the template to start one from.
func (p *themePicker) importDialog(gtx C, th *ui.Theme, t themeTasks) {
	d := &p.dialogs
	url := strings.TrimSpace(d.url.Text())
	submitted := false
	for {
		ev, more := d.url.Update(gtx)
		if !more {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	if (d.install.Clicked(gtx) || submitted) && url != "" && !t.importing {
		p.installRepository(url, false)
	}
	if d.choose.Clicked(gtx) && !t.importing {
		p.chooseFile()
	}
	if d.template.Clicked(gtx) && !t.importing {
		p.saveTemplate()
	}
	closing := d.close.Clicked(gtx)
	if t.focusURL && t.importOpen {
		gtx.Execute(key.FocusCmd{Tag: &d.url})
		p.with(func(t *themeTasks) { t.focusURL = false })
	}
	on := th.On(th.Popover)
	dlg := th.Dialog(&d.imp, t.importOpen, i18n.T("settings.importThemeDialog.title"), i18n.T("settings.importThemeDialog.description"))
	dlg.MaxWidth = importDialogWidth
	dlg.Body = func(gtx C) D { return p.importBody(gtx, on, url, t.importing) }
	dlg.Actions = []layout.Widget{func(gtx C) D { return p.importFooter(gtx, on, t.importing) }}
	if dlg.Layout(gtx) || closing {
		p.with(func(t *themeTasks) { t.importOpen = false })
	}
}

// importBody is the repository form, the "or" rule and the file row,
// gap-4 apart.
func (p *themePicker) importBody(gtx C, th *ui.Theme, url string, busy bool) D {
	d := &p.dialogs
	d.url.Submit = true
	install := th.Button(&d.install, i18n.T("settings.importThemeDialog.install"))
	if busy {
		install.Text = i18n.T("settings.importThemeDialog.installing")
	}
	install.Disabled = url == "" || busy
	field := th.Input(&d.url, importPlaceholder)
	field.Mono, field.TextSize, field.Disabled = true, ui.TextXS, busy
	hint := th.Text(ui.TextXS, i18n.T("settings.importThemeDialog.repositoryHint", i18n.Args{"file": repositoryManifest})).Muted()
	hint.MaxLines = 0
	choose := th.Button(&d.choose, i18n.T("settings.importThemeDialog.chooseFile"))
	choose.Variant, choose.Icon, choose.Disabled = ui.VariantOutline, icFileJSON, busy
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("settings.importThemeDialog.repositoryUrl")).Layout),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, field.Layout),
				ui.Gap(2),
				layout.Rigid(install.Layout),
			)
		}),
		ui.VGap(2),
		layout.Rigid(hint.Layout),
		ui.VGap(4),
		layout.Rigid(func(gtx C) D { return orRule(gtx, th) }),
		ui.VGap(4),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(choose.Layout),
				ui.Gap(3),
				layout.Flexed(1, th.Text(ui.TextXS, i18n.T("settings.importThemeDialog.fileHint")).Muted().Layout),
			)
		}),
	)
}

// orRule is the "—— or ——" between the two ways in.
func orRule(gtx C, th *ui.Theme) D {
	line := func(gtx C) D {
		return layout.Center.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Max.Y = gtx.Dp(1)
			return ruleAbove(gtx, th)
		})
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, line),
		ui.Gap(3),
		layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.importThemeDialog.or")).Muted().Layout),
		ui.Gap(3),
		layout.Flexed(1, line),
	)
}

// importFooter is DialogFooter's sm:justify-between: the template on the
// left, Cancel on the right.
func (p *themePicker) importFooter(gtx C, th *ui.Theme, busy bool) D {
	d := &p.dialogs
	template := th.Button(&d.template, i18n.T("settings.importThemeDialog.downloadTemplate"))
	template.Variant, template.Icon, template.Disabled = ui.VariantGhost, icDownload, busy
	cancel := th.Button(&d.close, i18n.T("common.action.cancel"))
	cancel.Variant, cancel.Disabled = ui.VariantGhost, busy
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
		layout.Rigid(template.Layout),
		layout.Rigid(cancel.Layout),
	)
}
