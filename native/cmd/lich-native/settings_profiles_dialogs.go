package main

import (
	"fmt"
	"image"
	"io"
	"math"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n (claudeProfileDialogs.*).
const (
	dlgNewTitle        = "New profile"
	dlgName            = "Name"
	dlgNamePlaceholder = "Work"
	dlgBring           = "Plugins to bring from %s"
	dlgNoPlugins       = "%s has no plugins to bring."
	dlgLichLocked      = "lich installs it in every profile"
	dlgPluginsHint     = "More can be copied later from the profile's menu. Your organization may refuse some of them."
	dlgCreateAndLogin  = "Create and log in"
	dlgRenameTitle     = "Rename profile"
	dlgRename          = "Rename"
	dlgReadyDesc       = "Pick it as the Profile of a project, or of every new session."
	dlgFailedDesc      = "Try again from the profile's menu, or log in from a terminal."
	dlgDeleteTitle     = "Delete %s?"
	dlgDelete          = "Delete"
	dlgCopyTitle       = "Copy plugins to %s"
	dlgCopyDesc        = "They are installed through Claude Code as this profile, so its account's rules apply."
	dlgCopyFrom        = "Copy from"
	dlgCopy            = "Copy plugins"
	dlgLoginTitle      = "Log in to %s"
	dlgWaiting         = "Waiting for the login…"
	dlgOpenBrowser     = "Open in browser"
	dlgCopyAddress     = "Copy"
	dlgPasteCode       = "Paste the code"
	dlgSubmitCode      = "Submit"
	dlgTerminal        = "Log in from a terminal instead"
	dlgReadyTitle      = "%s is ready"
	dlgFailedTitle     = "Could not log in to %s"
	dlgCopiedTitle     = "Plugins for %s"
	dlgLoggedInAs      = "Logged in as %s"
	dlgLoggedIn        = "Logged in"
	dlgLichPluginNote  = "The lich plugin could not be installed: %s"
	dlgDone            = "Done"
)

// The lich plugin is in every Claude Code profile whether or not the checklist
// is ticked: lich installs it after a login (claudeprofile.SetLichPlugin).
const lichPluginName = "lich"

var (
	dialogListMax = ui.Space(56)
	icDlgLoader   = icons.Lucide("loader-circle")
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
		return dlgNewTitle, prov.words.newDesc
	case flowRename:
		return dlgRenameTitle, prov.words.renameDesc
	case flowDelete:
		return fmt.Sprintf(dlgDeleteTitle, name), prov.words.deleteDesc
	case flowCopy:
		if d.step == stepResult {
			return fmt.Sprintf(dlgCopiedTitle, name), dlgCopyDesc
		}
		return fmt.Sprintf(dlgCopyTitle, name), dlgCopyDesc
	}
	switch {
	case d.step != stepResult:
		return fmt.Sprintf(dlgLoginTitle, name), prov.words.loginDesc
	case d.failure != "":
		return fmt.Sprintf(dlgFailedTitle, name), dlgFailedDesc
	}
	return fmt.Sprintf(dlgReadyTitle, name), dlgReadyDesc
}

// loggedInAs is the line a finished login opens its result with.
func loggedInAs(p lichclient.Profile) string {
	if p.Email == "" {
		return dlgLoggedIn
	}
	line := fmt.Sprintf(dlgLoggedInAs, p.Email)
	if p.Plan != "" {
		line += profileAccountSplit + strings.ToUpper(p.Plan[:1]) + p.Plan[1:]
	}
	return line
}

// layoutProfileDialogs lays out the profile dialog of each provider that has
// profiles; at most one is open.
func (s *settingsScreen) layoutProfileDialogs(gtx C, th *ui.Theme) {
	for _, kind := range []string{claudeKind, codexKind} {
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
	dlg.MaxWidth = ui.DialogWidthXL
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
	go func() {
		if err := c.provider.api(s.client).LoginCode(s.ctx, slug, code); err != nil {
			c.flow.set(func(d *flowData) { d.failure = lichclient.ErrorText(err) })
			s.invalidate()
		}
	}()
}

func (s *settingsScreen) flowActions(on *ui.Theme, f *profileFlow, d flowData) []layout.Widget {
	cancel := on.Button(&f.cancel, dialogCancel)
	cancel.Variant = ui.VariantGhost
	switch {
	case d.step == stepResult:
		return []layout.Widget{on.Button(&f.done, dlgDone).Layout}
	case d.step == stepWaiting || (d.step == stepWorking && d.kind == flowLogin):
		return []layout.Widget{cancel.Layout}
	case d.step == stepWorking:
		return nil
	}
	label := map[flowKind]string{flowNew: dlgCreateAndLogin, flowRename: dlgRename, flowDelete: dlgDelete, flowCopy: dlgCopy}[d.kind]
	confirm := on.Button(&f.confirm, label)
	confirm.Disabled = !s.canConfirm(d, f)
	if d.kind == flowDelete {
		confirm.Variant = ui.VariantDestructive
	}
	return []layout.Widget{cancel.Layout, confirm.Layout}
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
		layout.Rigid(th.Label(dlgName).Layout),
		ui.VGap(1.5),
		layout.Rigid(th.Input(name, dlgNamePlaceholder).Layout),
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
	lead := func(gtx C) D { return profileAvatar(gtx, th, name, d.source, profileAvatarSel) }
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(dlgCopyFrom).Layout),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			return pop.Layout(gtx, func(gtx C) D { return selectTriggerWith(gtx, th, lead, name, profilePickerW) })
		}),
	)
}

func (s *settingsScreen) sourceOption(gtx C, th *ui.Theme, f *profileFlow, p lichclient.Profile, chosen bool) D {
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
				layout.Rigid(func(gtx C) D { return profileAvatar(gtx, on, p.Name, p.Slug, profileAvatarSel) }),
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
	heading := fmt.Sprintf(dlgBring, source)
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
		children = append(children, layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf(dlgNoPlugins, source)).Muted().Layout))
	} else {
		children = append(children, layout.Rigid(func(gtx C) D { return s.pluginList(gtx, th, f, d.plugins) }))
	}
	if d.kind == flowNew {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, dlgPluginsHint).Muted()) }))
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
			layout.Flexed(1, th.Text(ui.TextXS, dlgLichLocked).Muted().Layout),
		)
	})
}

func (s *settingsScreen) pluginList(gtx C, th *ui.Theme, f *profileFlow, plugins []lichclient.ProfilePlugin) D {
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

func pluginRow(gtx C, th *ui.Theme, p lichclient.ProfilePlugin, box *widget.Bool) D {
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
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return spinnerLine(gtx, th, dlgWaiting) })}
	if d.url != "" {
		children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return s.addressBlock(gtx, th, f, d.url) }))
	}
	if f.ps.provider.loginCode {
		children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return s.codeBlock(gtx, th, f) }))
	}
	if d.failure != "" {
		children = append(children, ui.VGap(2), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, d.failure).In(th.Destructive)) }))
	}
	children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return terminalLink(gtx, th, &f.term) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *settingsScreen) addressBlock(gtx C, th *ui.Theme, f *profileFlow, url string) D {
	open := th.Button(&f.openURL, dlgOpenBrowser)
	open.Variant, open.Size, open.Icon = ui.VariantOutline, ui.SizeSM, icDlgOpen
	copyURL := th.Button(&f.copyURL, dlgCopyAddress)
	copyURL.Variant, copyURL.Size, copyURL.Icon = ui.VariantGhost, ui.SizeSM, icDlgCopy
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, f.ps.provider.words.addressDesc).Muted()) }),
		ui.VGap(1.5),
		layout.Rigid(th.Text(ui.TextXS, url).Mono().Layout),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, layout.Rigid(open.Layout), ui.Gap(2), layout.Rigid(copyURL.Layout))
		}),
	)
}

func (s *settingsScreen) codeBlock(gtx C, th *ui.Theme, f *profileFlow) D {
	submit := th.Button(&f.submitCode, dlgSubmitCode)
	submit.Variant, submit.Size = ui.VariantOutline, ui.SizeDefault
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(dlgPasteCode).Layout),
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
		return th.Text(ui.TextXS, dlgTerminal).In(ink).Layout(gtx)
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
		note := fmt.Sprintf(dlgLichPluginNote, d.pluginNote)
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, note).In(th.Destructive)) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func pluginResult(gtx C, th *ui.Theme, r lichclient.ProfilePluginResult) D {
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
	const lap = time.Second
	turn := float64(gtx.Now.UnixNano()%int64(lap)) / float64(lap)
	size := ui.Space(4)
	half := float32(gtx.Dp(size)) / 2
	gtx.Execute(op.InvalidateCmd{})
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(half, half), float32(turn*2*math.Pi))).Push(gtx.Ops).Pop()
			return icDlgLoader.Layout(gtx, size, th.MutedForeground)
		}),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, label).Muted().Weight(font.Normal).Layout),
	)
}
