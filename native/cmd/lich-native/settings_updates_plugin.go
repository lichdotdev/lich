package main

import (
	"log"
	"slices"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The loading toasts (installing, updating, updatingAll) wait on the toaster
// drawing a spinner; the button's own spinner stands in for them.
const pluginProviderIcon = 15 // ProviderIcon size={15}

// pluginHints are the catalog keys of what a provider's plugin cannot do,
// in its mark's tooltip while the plugin is installed.
var pluginHints = map[string]string{
	"crush":       "settings.settingsUpdatesPlugin.hint.crush",
	"omp":         "settings.settingsUpdatesPlugin.hint.omp",
	"antigravity": "settings.settingsUpdatesPlugin.hint.antigravity",
	"cursor":      "settings.settingsUpdatesPlugin.hint.cursor",
}

// pluginTodos are the catalog keys of a step the user takes in the provider
// itself, under its own line while the plugin is installed: lich cannot tell
// whether it was taken.
var pluginTodos = map[string]string{
	"codex": "settings.settingsUpdatesPlugin.hint.codex",
}

// pluginControls are the plugin block's buttons, one action per provider
// line, and the tooltips of the marks that need none.
type pluginControls struct {
	updateAll ui.Clickable
	actions   map[string]*ui.Clickable
	tips      map[string]*ui.Tooltip
}

// pluginNeedsLine is a provider with something for the user to do: an
// action to press or a step to take; the rest stand as marks in the title.
func pluginNeedsLine(st backend.PluginStatus) bool {
	_, _, act := pluginAction(st)
	_, todo := pluginTodos[st.Provider]
	return act || (todo && st.Installed)
}

// pluginTitle is the block's title with the release the rows install, the
// name alone until they are read.
func pluginTitle(statuses []backend.PluginStatus) string {
	title := i18n.T("settings.pluginSetting.title")
	for _, st := range statuses {
		if st.LatestVersion != "" {
			return title + " v" + st.LatestVersion
		}
	}
	return title
}

// pluginVersionText is what a row says about the plugin in that CLI.
func pluginVersionText(st backend.PluginStatus) string {
	switch {
	case !st.Available:
		return i18n.T("settings.pluginSetting.cliMissing")
	case !st.Installed:
		return i18n.T("settings.pluginSetting.notInstalled")
	case st.Compatible:
		return "v" + st.InstalledVersion
	}
	return i18n.T("settings.pluginSetting.unsupported", i18n.Args{"version": st.InstalledVersion})
}

// pluginAction is the button a row offers, none when there is nothing to
// close. An update is an outline button; an install is the surface's primary.
func pluginAction(st backend.PluginStatus) (label string, variant ui.Variant, ok bool) {
	switch {
	case st.Available && !st.Installed:
		return i18n.T("settings.pluginSetting.install"), ui.VariantDefault, true
	case st.Installed && st.Reregister:
		return i18n.T("settings.pluginSetting.reregister"), ui.VariantOutline, true
	case st.Installed && st.UpdateAvailable && st.Compatible:
		return i18n.T("settings.pluginSetting.updateTo", i18n.Args{"version": st.LatestVersion}), ui.VariantOutline, true
	case st.Installed && st.UpdateAvailable:
		return i18n.T("settings.pluginSetting.installVersion", i18n.Args{"version": st.LatestVersion}), ui.VariantOutline, true
	}
	return "", ui.VariantDefault, false
}

func outdatedPlugins(statuses []backend.PluginStatus) []backend.PluginStatus {
	return slices.DeleteFunc(slices.Clone(statuses), func(st backend.PluginStatus) bool {
		return !st.Installed || !st.UpdateAvailable
	})
}

// readPlugins asks for the rows again and files the answer, nil when the read
// failed; the last good rows stay on screen beside the failure.
func (u *updatesSettings) readPlugins(s *settingsScreen) []backend.PluginStatus {
	statuses, err := s.client.PluginStatuses(s.ctx)
	u.mu.Lock()
	u.pluginsFailed = err != nil
	if err == nil {
		u.plugins = statuses
	}
	u.mu.Unlock()
	if err != nil {
		log.Printf("settings: plugin status: %v", err)
	}
	s.invalidate()
	return statuses
}

func (u *updatesSettings) setBusy(s *settingsScreen, busy bool) {
	u.mu.Lock()
	u.busy = busy
	u.mu.Unlock()
	s.invalidate()
}

// act is a row's button: it asks again first and does what the fresh row is
// missing, so a plugin removed or installed from a terminal since the rows
// were read is neither updated nor installed twice.
func (u *updatesSettings) act(s *settingsScreen, provider string) {
	u.setBusy(s, true)
	defer u.setBusy(s, false)
	fresh := u.readPlugins(s)
	i := slices.IndexFunc(fresh, func(st backend.PluginStatus) bool { return st.Provider == provider })
	if i < 0 {
		return
	}
	switch row := fresh[i]; {
	case row.Available && !row.Installed:
		u.run(s, func() error { return s.client.InstallPlugin(s.ctx, provider) }, "settings.pluginSetting.installed", "settings.settingsUpdatesPlugin.installFailed")
	case row.Installed && row.UpdateAvailable:
		u.run(s, func() error { return s.client.UpdatePlugin(s.ctx, provider) }, "settings.pluginSetting.updated", "settings.settingsUpdatesPlugin.updateFailed")
	}
}

func (u *updatesSettings) updateAllOutdated(s *settingsScreen) {
	u.setBusy(s, true)
	defer u.setBusy(s, false)
	outdated := outdatedPlugins(u.readPlugins(s))
	u.run(s, func() error {
		for _, st := range outdated {
			if err := s.client.UpdatePlugin(s.ctx, st.Provider); err != nil {
				return err
			}
		}
		return nil
	}, "settings.pluginSetting.updated", "settings.settingsUpdatesPlugin.updateFailed")
}

// run is the action behind a toast, which says how it ended; done and failed
// are catalog keys, failed taking the error. The rows are read again after a
// success so the next visit shows what it changed.
func (u *updatesSettings) run(s *settingsScreen, call func() error, done, failed string) {
	if err := call(); err != nil {
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T(failed, i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	s.theme.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T(done)})
	u.readPlugins(s)
}

// pluginBlock is PluginSetting: the title with the marks of the providers
// that need nothing, and a line under it for each one that does.
func (u *updatesSettings) pluginBlock(gtx C, th *ui.Theme, s *settingsScreen, r settingRow, first bool) D {
	c := &u.plugin
	if c.updateAll.Clicked(gtx) {
		go u.updateAllOutdated(s)
	}
	u.mu.Lock()
	statuses, busy := u.plugins, u.busy
	u.mu.Unlock()
	if c.actions == nil {
		c.actions, c.tips = map[string]*ui.Clickable{}, map[string]*ui.Tooltip{}
	}
	for _, st := range statuses {
		if clickableOf(c.actions, st.Provider).Clicked(gtx) {
			go u.act(s, st.Provider)
		}
	}
	r.title = pluginTitle(statuses)
	var lines []layout.FlexChild
	for _, st := range statuses {
		if pluginNeedsLine(st) {
			lines = append(lines, layout.Rigid(func(gtx C) D { return c.row(gtx, th, st, busy) }))
		}
	}
	marks := func(gtx C, th *ui.Theme) D {
		return c.marks(gtx, th, s.root, statuses, len(outdatedPlugins(statuses)) > 1, busy)
	}
	return r.frame(gtx, th, first, func(gtx C, th *ui.Theme) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return r.content(gtx, th, marks) }),
			layout.Rigid(func(gtx C) D {
				if len(lines) == 0 {
					return D{}
				}
				return layout.Inset{Left: rowPadX, Right: rowPadX, Top: -rowPadY / 2, Bottom: rowPadY}.Layout(gtx, func(gtx C) D {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
				})
			}),
		)
	})
}

// marks are the providers that need nothing, each mark's tooltip naming it
// with its version, and Update all for more than one outdated plugin. A CLI
// that is not installed stands faded.
func (c *pluginControls) marks(gtx C, th *ui.Theme, root *ui.Root, statuses []backend.PluginStatus, offerAll, busy bool) D {
	var children []layout.FlexChild
	for _, st := range statuses {
		if pluginNeedsLine(st) {
			continue
		}
		tip := c.tips[st.Provider]
		if tip == nil {
			tip = &ui.Tooltip{}
			c.tips[st.Provider] = tip
		}
		children = append(children, ui.Gap(1.5), layout.Rigid(func(gtx C) D {
			return th.TooltipCard(root, tip, pluginTip(th.On(th.Popover), st)).Layout(gtx, func(gtx C) D {
				if !st.Available {
					defer paint.PushOpacity(gtx.Ops, rowOffOpacity).Pop()
				}
				return providerGlyph(gtx, th, st.Provider, unit.Dp(pluginProviderIcon), markPixel)
			})
		}))
	}
	if offerAll {
		all := th.Button(&c.updateAll, i18n.T("settings.pluginSetting.updateAll"))
		all.Size, all.Disabled = ui.SizeSM, busy
		children = append(children, ui.Gap(3), layout.Rigid(func(gtx C) D {
			if busy {
				return th.SpinnerBeside(gtx, all.Layout)
			}
			return all.Layout(gtx)
		}))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// pluginTip is a mark's tooltip: the provider, what it has installed and,
// while installed, what its plugin cannot do.
func pluginTip(th *ui.Theme, st backend.PluginStatus) layout.Widget {
	return func(gtx C) D {
		head := func(gtx C) D {
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, st.Name).Weight(font.Medium).Layout),
				ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextXS, pluginVersionText(st)).Muted().Layout),
			)
		}
		hint, ok := pluginHints[st.Provider]
		if !ok || !st.Installed {
			return head(gtx)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(head),
			ui.VGap(1),
			layout.Rigid(th.Text(ui.TextXS, i18n.T(hint)).Muted().Wrapped().Layout),
		)
	}
}

// row is one provider CLI that needs something: its mark and name, what it
// has installed, the action that closes the gap and the step it asks for.
func (c *pluginControls) row(gtx C, th *ui.Theme, st backend.PluginStatus, busy bool) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return providerGlyph(gtx, th, st.Provider, unit.Dp(pluginProviderIcon), markPixel)
		}),
		ui.Gap(2.5),
		layout.Rigid(th.Text(ui.TextSM, st.Name).Layout),
		layout.Flexed(1, func(gtx C) D {
			return layout.E.Layout(gtx, th.Text(ui.TextXS, pluginVersionText(st)).Muted().Layout)
		}),
	}
	if label, variant, ok := pluginAction(st); ok {
		b := th.Button(clickableOf(c.actions, st.Provider), label)
		b.Size, b.Variant, b.Disabled = ui.SizeSM, variant, busy
		children = append(children, ui.Gap(2.5), layout.Rigid(func(gtx C) D {
			if busy {
				return th.SpinnerBeside(gtx, b.Layout)
			}
			return b.Layout(gtx)
		}))
	}
	line := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
	todo, ok := pluginTodos[st.Provider]
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		if !ok || !st.Installed {
			return line(gtx)
		}
		// Under the name, past the mark.
		indent := unit.Dp(pluginProviderIcon) + ui.Space(2.5)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(line),
			layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: indent, Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
					return th.Text(ui.TextXS, i18n.T(todo)).Muted().Wrapped().Layout(gtx)
				})
			}),
		)
	})
}

// pluginProfilesFailed raises a toast per profile a pass could not bring to
// Personal's lich plugin; a pass with none says nothing, as Personal's own
// update already did.
func (u *window) pluginProfilesFailed(pass backend.PluginProfiles) {
	for _, f := range pass.Failures {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.settingsUpdatesPlugin.profileFailed", i18n.Args{"pass": pass.Name, "profile": f.Name, "error": f.Error})})
	}
}
