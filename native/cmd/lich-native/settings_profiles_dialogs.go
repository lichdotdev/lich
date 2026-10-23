package main

import (
	"image"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// The lich plugin is in every Claude Code and Codex profile whether or not the
// checklist is ticked: lich installs it after a login (SetLichPlugin of
// claudeprofile and codexprofile).
const lichPluginName = "lich"

var (
	dialogListMax = ui.Space(56)
	icDlgOK       = icons.Lucide("check")
	icDlgBan      = icons.Lucide("ban")
	icDlgOpen     = icons.Lucide("external-link")
	icDlgCopy     = icons.Lucide("copy")
)

// flowHeading is the dialog's title and description for where it stands.
func flowHeading(prov profileProvider, d flowData) (title, desc string) {
	name := d.target.Name
	switch d.kind {
	case flowNew:
		return i18n.T("settings.settingsProfilesView.newProfile"), profileSentence(prov, prov.words.newDescKey)
	case flowRename:
		return i18n.T("settings.settingsProfilesDialogs.renameTitle"), profileSentence(prov, prov.words.renameDescKey)
	case flowDelete:
		return i18n.T("settings.settingsProfilesDialogs.deleteTitle", i18n.Args{"name": name}), profileSentence(prov, prov.words.deleteDescKey)
	case flowCopy:
		desc := i18n.T("settings.settingsProfilesDialogs.copyDesc", i18n.Args{"provider": prov.label})
		if d.step == stepResult {
			return i18n.T("settings.settingsProfilesDialogs.copiedTitle", i18n.Args{"name": name}), desc
		}
		return i18n.T("settings.settingsProfilesDialogs.copyTitle", i18n.Args{"name": name}), desc
	}
	switch {
	case d.step != stepResult:
		return i18n.T("settings.settingsProfilesDialogs.loginTitle", i18n.Args{"name": name}), profileSentence(prov, prov.words.loginDescKey)
	case d.failure != "":
		return i18n.T("settings.settingsProfilesDialogs.failedTitle", i18n.Args{"name": name}), i18n.T("settings.settingsProfilesDialogs.failedDesc")
	}
	return i18n.T("settings.settingsProfilesDialogs.readyTitle", i18n.Args{"name": name}), i18n.T("settings.settingsProfilesDialogs.readyDesc")
}

// loggedInAs is the line a finished login opens its result with.
func loggedInAs(p backend.Profile) string {
	if p.Email == "" {
		return i18n.T("settings.profiles.loggedIn")
	}
	line := i18n.T("settings.settingsProfilesDialogs.loggedInAs", i18n.Args{"email": p.Email})
	if p.Plan != "" {
		line += profileAccountSplit + strings.ToUpper(p.Plan[:1]) + p.Plan[1:]
	}
	return line
}

// layoutProfileDialogs lays out the profile dialog of each provider that has
// profiles; at most one is open.
func (s *settingsScreen) layoutProfileDialogs(gtx C, th *ui.Theme) {
	for _, kind := range append([]string{claudeKind, codexKind, ompKind}, xdgProfileKinds()...) {
		if c := s.providers.profiles(kind); c != nil {
			s.layoutProfileDialog(gtx, th, c)
		}
	}
}

// layoutProfileDialog is a provider's profile dialog. The window lays it out
// with its own constraints, ahead of the pane, so a click that opens it calls
// s.invalidate.
func (s *settingsScreen) layoutProfileDialog(gtx C, th *ui.Theme, c *profileSettings) {
	f := &c.flow
	s.flowInput(gtx, f, f.view())
	d := f.view()
	on := th.On(th.Popover)
	title, desc := flowHeading(c.provider, d)
	dlg := th.Dialog(&f.dialog, d.open && s.open, title, desc)
	dlg.MaxWidth, dlg.Root = ui.DialogWidthXL, s.root
	dlg.Body = func(gtx C) D { return s.flowBody(gtx, on, f, d) }
	dlg.Actions = s.flowActions(on, f, d)
	if dlg.Layout(gtx) {
		s.dismissFlow(c, d)
	}
}

// flowInput reads what the dialog's widgets were asked this frame.
func (s *settingsScreen) flowInput(gtx C, f *profileFlow, d flowData) {
	if d.fresh {
		f.code.SetText("")
		f.checks = nil
		f.set(func(data *flowData) { data.fresh = false })
	}
	if d.refresh {
		f.set(func(data *flowData) { data.refresh = false })
		s.refresh()
	}
	f.name.Submit, f.code.Submit = true, true
	named, coded := editorSubmitted(gtx, &f.name), editorSubmitted(gtx, &f.code)
	if f.focus && d.open {
		gtx.Execute(key.FocusCmd{Tag: &f.name})
		f.focus = false
	}
	switch {
	case f.cancel.Clicked(gtx):
		s.dismissFlow(f.ps, d)
	case f.done.Clicked(gtx):
		s.dismissFlow(f.ps, d)
	case f.confirm.Clicked(gtx) || named:
		if d.step == stepForm && s.canConfirm(d, f) {
			s.confirmFlow(f.ps, d)
		}
	case f.submitCode.Clicked(gtx) || coded:
		s.submitLoginCode(f.ps, d.target.Slug, f.code.Text())
		f.code.SetText("")
	case f.openURL.Clicked(gtx):
		s.openExternal(d.url)
	case f.copyURL.Clicked(gtx):
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(d.url))})
	case f.term.Clicked(gtx):
		s.loginInTerminal(f.ps, d)
	}
}

func editorSubmitted(gtx C, e *widget.Editor) bool {
	submitted := false
	for {
		ev, more := e.Update(gtx)
		if !more {
			return submitted
		}
		if _, is := ev.(widget.SubmitEvent); is {
			submitted = true
		}
	}
}

// canConfirm is whether the form has what its action needs.
func (s *settingsScreen) canConfirm(d flowData, f *profileFlow) bool {
	switch d.kind {
	case flowNew, flowRename:
		return strings.TrimSpace(f.name.Text()) != ""
	case flowCopy:
		return len(f.checkedKeys(d.plugins)) > 0
	}
	return true
}

// submitLoginCode hands the code the sign-in page showed to the login.
func (s *settingsScreen) submitLoginCode(c *profileSettings, slug, code string) {
	code = strings.TrimSpace(code)
	if code == "" || !c.provider.loginCode {
		return
	}
	c.flow.set(func(d *flowData) { d.authenticating, d.failure = true, "" })
	go func() {
		if err := c.provider.api(s.client).LoginCode(s.ctx, slug, code); err != nil {
			c.flow.set(func(d *flowData) { d.authenticating, d.failure = false, backend.ErrorText(err) })
			s.invalidate()
		}
	}()
}

func (s *settingsScreen) flowActions(on *ui.Theme, f *profileFlow, d flowData) []layout.Widget {
	cancel := on.Button(&f.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	switch {
	case d.step == stepResult:
		return []layout.Widget{on.Button(&f.done, i18n.T("settings.settingsProfilesDialogs.done")).Layout}
	case d.step == stepWaiting || (d.step == stepWorking && d.kind == flowLogin):
		return []layout.Widget{cancel.Layout}
	case d.step == stepWorking:
		return nil
	}
	confirm := on.Button(&f.confirm, confirmLabel(d.kind))
	confirm.Disabled = !s.canConfirm(d, f)
	if d.kind == flowDelete {
		confirm.Variant = ui.VariantDestructive
	}
	return []layout.Widget{cancel.Layout, confirm.Layout}
}

func confirmLabel(kind flowKind) string {
	switch kind {
	case flowNew:
		return i18n.T("settings.settingsProfilesDialogs.createAndLogin")
	case flowRename:
		return i18n.T("settings.settingsProfilesView.rename")
	case flowDelete:
		return i18n.T("settings.settingsProfilesView.delete")
	}
	return i18n.T("settings.settingsProfilesDialogs.copy")
}

func (s *settingsScreen) flowBody(gtx C, th *ui.Theme, f *profileFlow, d flowData) D {
	switch {
	case d.step == stepWorking:
		return spinnerLine(gtx, th, d.working)
	case d.step == stepWaiting:
		return s.loginBody(gtx, th, f, d)
	case d.step == stepResult:
		return s.resultBody(gtx, th, d)
	}
	return s.formBody(gtx, th, f, d)
}

func (s *settingsScreen) formBody(gtx C, th *ui.Theme, f *profileFlow, d flowData) D {
	var children []layout.FlexChild
	switch d.kind {
	case flowNew, flowRename:
		children = append(children, layout.Rigid(func(gtx C) D { return nameField(gtx, th, &f.name) }))
	case flowCopy:
		children = append(children, layout.Rigid(func(gtx C) D { return s.copySource(gtx, th, f, d) }), ui.VGap(3))
	}
	if f.ps.provider.plugins && (d.kind == flowNew || d.kind == flowCopy) {
		children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return s.checklist(gtx, th, f, d) }))
	}
	if d.failure != "" {
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, d.failure).In(th.Destructive)) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func nameField(gtx C, th *ui.Theme, name *widget.Editor) D {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("settings.settingsProfilesDialogs.name")).Layout),
		ui.VGap(1.5),
		layout.Rigid(th.Input(name, i18n.T("settings.settingsProfilesDialogs.namePlaceholder")).Layout),
	)
}

// copySource is the select of the profile a copy reads from, any but the
// target.
func (s *settingsScreen) copySource(gtx C, th *ui.Theme, f *profileFlow, d flowData) D {
	book := f.ps.book
	list, _ := book.snapshot()
	pop := th.DropdownPanel(s.root, &f.source, profilePickerW, func(gtx C, th *ui.Theme) D {
		var rows []layout.FlexChild
		for _, p := range list {
			if p.Slug == d.target.Slug {
				continue
			}
			rows = append(rows, layout.Rigid(func(gtx C) D { return s.sourceOption(gtx, th, f, p, p.Slug == d.source) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
	name := book.nameOf(d.source)
	lead := func(gtx C) D { return profileAvatar(gtx, th, book.look(d.source), profileAvatarSel) }
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("settings.settingsProfilesDialogs.copyFrom")).Layout),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			return pop.Layout(gtx, func(gtx C) D { return selectTriggerWith(gtx, th, lead, name, profilePickerW) })
		}),
	)
}

func (s *settingsScreen) sourceOption(gtx C, th *ui.Theme, f *profileFlow, p backend.Profile, chosen bool) D {
	if f.options == nil {
		f.options = map[string]*widget.Clickable{}
	}
	click := clickableOf(f.options, p.Slug)
	if click.Clicked(gtx) {
		s.pickCopySource(f.ps, p.Slug)
		f.source.Close()
	}
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(fontRowHeight)))
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return profileAvatar(gtx, on, f.ps.book.look(p.Slug), profileAvatarSel) }),
				ui.Gap(2),
				layout.Flexed(1, on.Text(ui.TextSM, p.Name).Layout),
				layout.Rigid(func(gtx C) D {
					if !chosen {
						return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)}
					}
					return icCheck.Layout(gtx, ui.Space(4), on.Foreground)
				}),
			)
		})
	})
}

// checklist is the plugins a profile can take from the one it is copied from,
// each ticked until unticked, and for a new profile the lich plugin locked on.
func (s *settingsScreen) checklist(gtx C, th *ui.Theme, f *profileFlow, d flowData) D {
	source := f.ps.book.nameOf(d.source)
	heading := i18n.T("settings.settingsProfilesDialogs.bring", i18n.Args{"name": source})
	if d.kind == flowCopy {
		heading = ""
	}
	children := []layout.FlexChild{}
	if heading != "" {
		children = append(children, layout.Rigid(th.Label(heading).Layout), ui.VGap(1.5))
	}
	if d.kind == flowNew {
		children = append(children, layout.Rigid(func(gtx C) D { return lockedPlugin(gtx, th) }))
	}
	if len(d.plugins) == 0 {
		children = append(children, layout.Rigid(th.Text(ui.TextXS, i18n.T("settings.settingsProfilesDialogs.noPlugins", i18n.Args{"name": source})).Muted().Layout))
	} else {
		children = append(children, layout.Rigid(func(gtx C) D { return s.pluginList(gtx, th, f, d.plugins) }))
	}
	if d.kind == flowNew {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D {
			return wrapped(gtx, th.Text(ui.TextXS, i18n.T("settings.settingsProfilesDialogs.pluginsHint")).Muted())
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// lockedPlugin is the lich plugin's row: ticked and not to be unticked.
func lockedPlugin(gtx C, th *ui.Theme) D {
	on := widget.Bool{Value: true}
	box := th.Checkbox(&on)
	box.Disabled = true
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(box.Layout),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextSM, lichPluginName).Layout),
			ui.Gap(2),
			layout.Flexed(1, th.Text(ui.TextXS, i18n.T("settings.settingsProfilesDialogs.lichLocked")).Muted().Layout),
		)
	})
}

func (s *settingsScreen) pluginList(gtx C, th *ui.Theme, f *profileFlow, plugins []backend.ProfilePlugin) D {
	if f.checks == nil {
		f.checks = map[string]*widget.Bool{}
	}
	f.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(dialogListMax))
	return th.ScrollArea(&f.list).Layout(gtx, len(plugins), func(gtx C, i int) D {
		p := plugins[i]
		box := f.checks[p.Key]
		if box == nil {
			box = &widget.Bool{Value: true}
			f.checks[p.Key] = box
		}
		return pluginRow(gtx, th, p, box)
	})
}

func pluginRow(gtx C, th *ui.Theme, p backend.ProfilePlugin, box *widget.Bool) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Checkbox(box).Layout),
			ui.Gap(2),
			layout.Rigid(th.Text(ui.TextSM, p.Name).Layout),
			ui.Gap(2),
			layout.Flexed(1, th.Text(ui.TextXS, p.Marketplace).Muted().Mono().Layout),
		)
	})
}

// loginBody is the wait: the spinner, the address the CLI printed with its
// buttons, the field for the code the page shows, and the way out to a
// terminal.
func (s *settingsScreen) loginBody(gtx C, th *ui.Theme, f *profileFlow, d flowData) D {
	waiting := i18n.T("settings.settingsProfilesDialogs.waiting")
	if d.authenticating {
		waiting = i18n.T("settings.settingsProfilesDialogs.authenticating")
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return spinnerLine(gtx, th, waiting) })}
	if d.url != "" {
		children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return s.addressBlock(gtx, th, f, d.url) }))
	}
	if f.ps.provider.loginCode && !d.authenticating {
		children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return s.codeBlock(gtx, th, f) }))
	}
	if d.failure != "" {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, d.failure).In(th.Destructive)) }))
	}
	children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return terminalLink(gtx, th, &f.term) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *settingsScreen) addressBlock(gtx C, th *ui.Theme, f *profileFlow, url string) D {
	open := th.Button(&f.openURL, i18n.T("settings.settingsProfilesDialogs.openBrowser"))
	open.Variant, open.Size, open.Icon = ui.VariantOutline, ui.SizeSM, icDlgOpen
	copyURL := th.Button(&f.copyURL, i18n.T("settings.settingsProfilesDialogs.copyAddress"))
	copyURL.Variant, copyURL.Size, copyURL.Icon = ui.VariantGhost, ui.SizeSM, icDlgCopy
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return wrapped(gtx, th.Text(ui.TextXS, profileSentence(f.ps.provider, f.ps.provider.words.addressDescKey)).Muted())
		}),
		ui.VGap(1.5),
		layout.Rigid(th.Text(ui.TextXS, url).Mono().Layout),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(open.Layout), ui.Gap(2), layout.Rigid(copyURL.Layout))
		}),
	)
}

func (s *settingsScreen) codeBlock(gtx C, th *ui.Theme, f *profileFlow) D {
	submit := th.Button(&f.submitCode, i18n.T("settings.settingsProfilesDialogs.submitCode"))
	submit.Variant, submit.Size = ui.VariantOutline, ui.SizeDefault
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("settings.settingsProfilesDialogs.pasteCode")).Layout),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, th.Input(&f.code, "").Layout),
				ui.Gap(2),
				layout.Rigid(submit.Layout),
			)
		}),
	)
}

func terminalLink(gtx C, th *ui.Theme, click *widget.Clickable) D {
	return click.Layout(gtx, func(gtx C) D {
		ink := th.MutedForeground
		if click.Hovered() {
			ink = th.Foreground
		}
		return th.Text(ui.TextXS, i18n.T("settings.settingsProfilesDialogs.terminal")).In(ink).Layout(gtx)
	})
}

// resultBody is how a login or a copy ended: the account, then each plugin
// with the provider's own words when it was refused.
func (s *settingsScreen) resultBody(gtx C, th *ui.Theme, d flowData) D {
	var children []layout.FlexChild
	switch {
	case d.failure != "":
		children = append(children, layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextSM, d.failure).In(th.Destructive)) }))
	case d.account != "":
		children = append(children, layout.Rigid(th.Text(ui.TextSM, d.account).Layout))
	}
	for _, r := range d.results {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return pluginResult(gtx, th, r) }))
	}
	if d.pluginNote != "" {
		note := i18n.T("settings.settingsProfilesDialogs.lichPluginNote", i18n.Args{"error": d.pluginNote})
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, note).In(th.Destructive)) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func pluginResult(gtx C, th *ui.Theme, r backend.ProfilePluginResult) D {
	mark, ink := icDlgOK, th.TonePass
	if !r.OK {
		mark, ink = icDlgBan, th.Destructive
	}
	name, _, _ := strings.Cut(r.Key, "@")
	lines := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, name).Layout)}
	if r.Message != "" {
		lines = append(lines, layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, r.Message).Muted()) }))
	}
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx C) D { return mark.Layout(gtx, ui.Space(4), ink) })
		}),
		ui.Gap(2),
		layout.Flexed(1, func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...) }),
	)
}

// spinnerLine is LoaderCircle turning one lap a second beside a muted label.
func spinnerLine(gtx C, th *ui.Theme, label string) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ui.Spinner(gtx, ui.Space(4), th.MutedForeground) }),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, label).Muted().Weight(font.Normal).Layout),
	)
}
