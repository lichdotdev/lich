package main

import (
	"log"
	"slices"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// The loading toasts (installing, updating, updatingAll) wait on the toaster
// drawing a spinner; the button's own spinner stands in for them.
const pluginProviderIcon = 15 // ProviderIcon size={15}

// pluginHints are the catalog keys of the notes under a provider's row, shown
// while its plugin is installed.
var pluginHints = map[string]string{
	"codex":       "settings.settingsUpdatesPlugin.hint.codex",
	"crush":       "settings.settingsUpdatesPlugin.hint.crush",
	"omp":         "settings.settingsUpdatesPlugin.hint.omp",
	"antigravity": "settings.settingsUpdatesPlugin.hint.antigravity",
	"cursor":      "settings.settingsUpdatesPlugin.hint.cursor",
}

// pluginControls are the plugin block's buttons: one action per provider row.
type pluginControls struct {
	check, updateAll widget.Clickable
	actions          map[string]*widget.Clickable
}

// pluginVersionText is what a row says about the plugin in that CLI.
func pluginVersionText(st lichclient.PluginStatus) string {
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
func pluginAction(st lichclient.PluginStatus) (label string, variant ui.Variant, ok bool) {
	switch {
	case st.Available && !st.Installed:
		return i18n.T("settings.pluginSetting.install"), ui.VariantDefault, true
	case st.Installed && st.UpdateAvailable && st.Compatible:
		return i18n.T("settings.pluginSetting.updateTo", i18n.Args{"version": st.LatestVersion}), ui.VariantOutline, true
	case st.Installed && st.UpdateAvailable:
		return i18n.T("settings.pluginSetting.installVersion", i18n.Args{"version": st.LatestVersion}), ui.VariantOutline, true
	}
	return "", ui.VariantDefault, false
}

func outdatedPlugins(statuses []lichclient.PluginStatus) []lichclient.PluginStatus {
	return slices.DeleteFunc(slices.Clone(statuses), func(st lichclient.PluginStatus) bool {
		return !st.Installed || !st.UpdateAvailable
	})
}

// readPlugins asks for the rows again and files the answer, nil when the read
// failed; the last good rows stay on screen beside the failure.
func (u *updatesSettings) readPlugins(s *settingsScreen) []lichclient.PluginStatus {
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
	i := slices.IndexFunc(fresh, func(st lichclient.PluginStatus) bool { return st.Provider == provider })
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

// check re-reads the rows on request; the outcome is what the read left in
// pluginsFailed, so a later failure cannot sit under an older "Checked.".
func (u *updatesSettings) check(s *settingsScreen) {
	u.setBusy(s, true)
	defer u.setBusy(s, false)
	u.mu.Lock()
	u.checked = false
	u.mu.Unlock()
	u.readPlugins(s)
	u.mu.Lock()
	u.checked = true
	u.mu.Unlock()
}

// run is the action behind a toast, which says how it ended; done and failed
// are catalog keys, failed taking the error. The rows are read again after a
// success so the next visit shows what it changed.
func (u *updatesSettings) run(s *settingsScreen, call func() error, done, failed string) {
	if err := call(); err != nil {
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T(failed, i18n.Args{"error": lichclient.ErrorText(err)})})
		return
	}
	s.theme.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T(done)})
	u.readPlugins(s)
}

// pluginBlock is PluginSetting: a row per provider CLI, each with its note
// while its plugin is installed, and the check.
func (u *updatesSettings) pluginBlock(gtx C, th *ui.Theme, s *settingsScreen, r settingRow, first bool) D {
	r.description = i18n.T("settings.pluginSetting.description")
	c := &u.plugin
	if c.check.Clicked(gtx) {
		go u.check(s)
	}
	if c.updateAll.Clicked(gtx) {
		go u.updateAllOutdated(s)
	}
	u.mu.Lock()
	statuses, busy, failed, checked := u.plugins, u.busy, u.pluginsFailed, u.checked
	u.mu.Unlock()
	if c.actions == nil {
		c.actions = map[string]*widget.Clickable{}
	}
	for _, st := range statuses {
		if clickableOf(c.actions, st.Provider).Clicked(gtx) {
			go u.act(s, st.Provider)
		}
	}
	return r.layoutBlock(gtx, th, first, func(gtx C, th *ui.Theme) D {
		var children []layout.FlexChild
		for _, st := range statuses {
			children = append(children, layout.Rigid(func(gtx C) D { return c.row(gtx, th, st, busy) }))
		}
		children = append(children, ui.VGap(1), layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return c.footer(gtx, th, len(outdatedPlugins(statuses)) > 1, busy, pluginOutcome(checked, failed))
			})
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// pluginOutcome is what the last check says, "" until one was asked for.
func pluginOutcome(checked, failed bool) string {
	switch {
	case !checked:
		return ""
	case failed:
		return i18n.T("settings.pluginSetting.checkFailed")
	}
	return i18n.T("settings.pluginSetting.checked")
}

// row is one provider CLI: its mark and name, what it has installed, and the
// action that closes the gap.
func (c *pluginControls) row(gtx C, th *ui.Theme, st lichclient.PluginStatus, busy bool) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			return providerIcon(st.Provider).Layout(gtx, unit.Dp(pluginProviderIcon), th.Foreground)
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
		children = append(children, ui.Gap(2.5), layout.Rigid(func(gtx C) D { return busyButton(gtx, th, b, busy) }))
	}
	line := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
	hint, ok := pluginHints[st.Provider]
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
					return wrapped(gtx, th.Text(ui.TextXS, i18n.T(hint)).Muted())
				})
			}),
		)
	})
}

// footer is the check, the update-all for more than one outdated plugin, and
// the outcome.
func (c *pluginControls) footer(gtx C, th *ui.Theme, offerAll, busy bool, outcome string) D {
	check := th.Button(&c.check, i18n.T("settings.pluginSetting.checkForUpdates"))
	check.Size, check.Variant, check.Disabled = ui.SizeSM, ui.VariantOutline, busy
	children := []layout.FlexChild{layout.Rigid(check.Layout)}
	if offerAll {
		all := th.Button(&c.updateAll, i18n.T("settings.pluginSetting.updateAll"))
		all.Size, all.Disabled = ui.SizeSM, busy
		children = append(children, ui.Gap(3), layout.Rigid(func(gtx C) D { return busyButton(gtx, th, all, busy) }))
	}
	if outcome != "" {
		children = append(children, ui.Gap(3), layout.Rigid(th.Text(ui.TextXS, outcome).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// pluginProfilesFailed raises a toast per profile a pass could not bring to
// Personal's lich plugin; a pass with none says nothing, as Personal's own
// update already did.
func (u *window) pluginProfilesFailed(pass lichclient.PluginProfiles) {
	for _, f := range pass.Failures {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.settingsUpdatesPlugin.profileFailed", i18n.Args{"pass": pass.Name, "profile": f.Name, "error": f.Error})})
	}
}
