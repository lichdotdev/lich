package main

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// The web's plugin-gate.ts prefs, global settings here.
const (
	pluginInstallDismissedKey = "lich.pluginInstallDismissed"
	pluginUpdateDismissedKey  = "lich.pluginUpdateDismissed"
	pluginDismissedFlag       = "1"
	// pluginNoReportProvider runs Claude Code's plugin and reports its
	// version; the fix is offered on Claude Code's row.
	pluginNoReportProvider = "cursor"
)

type pluginActionKind uint8

const (
	pluginActionNone pluginActionKind = iota
	pluginActionIncompatible
	pluginActionInstall
	pluginActionUpdate
)

type pluginGateAction struct {
	kind      pluginActionKind
	version   string
	providers []lichclient.PluginStatus
}

// decidePluginAction is plugin-gate.ts's: an incompatible install wins and
// ignores every dismissal; a CLI missing the plugin comes before an update.
func decidePluginAction(statuses []lichclient.PluginStatus, installDismissed bool, updateDismissed string) pluginGateAction {
	available := slices.DeleteFunc(slices.Clone(statuses), func(s lichclient.PluginStatus) bool { return !s.Available })
	var incompatible, missing, outdated []lichclient.PluginStatus
	for _, s := range available {
		switch {
		case s.Installed && !s.Compatible && s.Provider != pluginNoReportProvider:
			incompatible = append(incompatible, s)
		case !s.Installed:
			missing = append(missing, s)
		}
		if s.Installed && s.UpdateAvailable {
			outdated = append(outdated, s)
		}
	}
	if len(incompatible) > 0 {
		return pluginGateAction{pluginActionIncompatible, incompatible[0].LatestVersion, incompatible}
	}
	if len(missing) > 0 && !installDismissed {
		return pluginGateAction{kind: pluginActionInstall, providers: missing}
	}
	if len(outdated) > 0 && updateDismissed != outdated[0].LatestVersion {
		return pluginGateAction{pluginActionUpdate, outdated[0].LatestVersion, outdated}
	}
	return pluginGateAction{}
}

func incompatibleMessage(version string, providers []lichclient.PluginStatus) string {
	installs := make([]string, len(providers))
	for i, p := range providers {
		installs[i] = fmt.Sprintf("%s (v%s)", p.Name, p.InstalledVersion)
	}
	if version == "" {
		return i18n.T("update.plugin.incompatibleUpdate", i18n.Args{"installs": strings.Join(installs, ", ")})
	}
	return i18n.T("update.plugin.incompatibleInstall", i18n.Args{"installs": strings.Join(installs, ", "), "version": version})
}

// pluginGate is AgentPluginGate: once per start, after the first-run dialog
// stored a default, and again whenever a session reports from a release this
// lich does not speak. Any failure is silent: it never blocks startup.
type pluginGate struct {
	started bool

	mu         sync.Mutex
	offer      []lichclient.PluginStatus
	installing bool

	picked                   map[string]*widget.Bool
	rows                     map[string]*widget.Clickable
	dialog                   ui.DialogState
	dontAsk, notNow, install widget.Clickable
}

// pollPluginGate starts the check once the provider book has a stored default.
func (u *window) pollPluginGate() {
	g := &u.plugin
	if g.started {
		return
	}
	m := u.model.providers.snapshot()
	if !m.loaded || m.globals[providerDefaultKey] == "" {
		return
	}
	g.started = true
	go u.checkPlugin()
}

func (u *window) checkPlugin() {
	var enabled []string
	for _, p := range u.model.providers.snapshot().enabled() {
		enabled = append(enabled, p.id)
	}
	statuses, err := u.client.PluginStatuses(u.ctx)
	if err != nil {
		log.Printf("plugin gate: %v", err)
		return
	}
	statuses = slices.DeleteFunc(statuses, func(s lichclient.PluginStatus) bool { return !slices.Contains(enabled, s.Provider) })
	installDismissed, err := u.client.GetSetting(u.ctx, pluginInstallDismissedKey, "")
	if err != nil {
		log.Printf("plugin gate: %v", err)
		return
	}
	updateDismissed, err := u.client.GetSetting(u.ctx, pluginUpdateDismissedKey, "")
	if err != nil {
		log.Printf("plugin gate: %v", err)
		return
	}
	a := decidePluginAction(statuses, installDismissed == pluginDismissedFlag, updateDismissed)
	switch a.kind {
	case pluginActionIncompatible:
		t := ui.Toast{ID: lichclient.PluginIncompatibleEvent, Variant: ui.ToastWarning,
			Title: incompatibleMessage(a.version, a.providers), Duration: ui.ToastForever}
		if a.version != "" {
			t.Action = &ui.ToastButton{Label: i18n.T("shell.agentPluginGate.installVersion", i18n.Args{"version": a.version}), OnClick: func() { go u.updatePlugins(a.providers) }}
		}
		u.toaster.Show(t)
	case pluginActionInstall:
		g := &u.plugin
		g.mu.Lock()
		g.offer = a.providers
		g.mu.Unlock()
		u.win.Invalidate()
	case pluginActionUpdate:
		u.toaster.Show(u.pluginUpdateToast(a))
	}
}

// pluginUpdateToast offers a's release, Later remembering it was declined.
func (u *window) pluginUpdateToast(a pluginGateAction) ui.Toast {
	return ui.Toast{Title: i18n.T("shell.agentPluginGate.updateAvailable", i18n.Args{"version": a.version}), Duration: ui.ToastForever,
		Action: &ui.ToastButton{Label: i18n.T("shell.agentPluginGate.update"), OnClick: func() { go u.updatePlugins(a.providers) }},
		Cancel: &ui.ToastButton{Label: i18n.T("shell.agentPluginGate.later"), OnClick: func() { u.writePref(pluginUpdateDismissedKey, a.version) }}}
}

// updatePlugins runs one update per provider in order, stopping at the first
// that fails so the error names the CLI that refused.
func (u *window) updatePlugins(providers []lichclient.PluginStatus) {
	for _, p := range providers {
		if err := u.client.UpdatePlugin(u.ctx, p.Provider); err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("update.progress.updateFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
			return
		}
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("shell.agentPluginGate.updated")})
}

func (u *window) installPlugins(providers []string) {
	g := &u.plugin
	defer func() {
		g.mu.Lock()
		g.installing = false
		g.mu.Unlock()
		u.win.Invalidate()
	}()
	for _, p := range providers {
		if err := u.client.InstallPlugin(u.ctx, p); err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.agentPluginGate.installFailed", i18n.Args{"error": lichclient.ErrorText(err)})})
			return
		}
	}
	g.mu.Lock()
	g.offer = nil
	g.mu.Unlock()
	u.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("shell.agentPluginGate.installed")})
	if slices.Contains(providers, codexKind) {
		u.toaster.Show(ui.Toast{Variant: ui.ToastInfo, Title: i18n.T("shell.agentPluginGate.codexTrust"), Duration: ui.ToastForever})
	}
}

func (u *window) writePref(key, value string) {
	go func() { u.logErr("write "+key, u.client.SetSetting(u.ctx, key, "", value)) }()
}

// layoutPluginGate is the install modal, a checkbox per CLI, all ticked.
func (u *window) layoutPluginGate(gtx C, th *ui.Theme) {
	g := &u.plugin
	u.pollPluginGate()
	g.mu.Lock()
	offer, installing := g.offer, g.installing
	g.mu.Unlock()
	closeOffer := func() {
		g.mu.Lock()
		g.offer = nil
		g.mu.Unlock()
	}
	if g.picked == nil {
		g.picked, g.rows = map[string]*widget.Bool{}, map[string]*widget.Clickable{}
	}
	var picked []string
	for _, p := range offer {
		if g.picked[p.Provider] == nil {
			g.picked[p.Provider], g.rows[p.Provider] = &widget.Bool{Value: true}, new(widget.Clickable)
		}
		if g.rows[p.Provider].Clicked(gtx) && !installing {
			g.picked[p.Provider].Value = !g.picked[p.Provider].Value
		}
		if g.picked[p.Provider].Value {
			picked = append(picked, p.Provider)
		}
	}
	if g.dontAsk.Clicked(gtx) && !installing {
		u.writePref(pluginInstallDismissedKey, pluginDismissedFlag)
		closeOffer()
	}
	if g.notNow.Clicked(gtx) && !installing {
		closeOffer()
	}
	if g.install.Clicked(gtx) && !installing && len(picked) > 0 {
		g.mu.Lock()
		g.installing, installing = true, true
		g.mu.Unlock()
		go u.installPlugins(picked)
	}
	on := th.On(th.Popover)
	dontAsk, notNow := on.Button(&g.dontAsk, i18n.T("shell.agentPluginGate.dontAskAgain")), on.Button(&g.notNow, i18n.T("shell.agentPluginGate.notNow"))
	dontAsk.Variant, notNow.Variant = ui.VariantGhost, ui.VariantGhost
	dontAsk.Disabled, notNow.Disabled = installing, installing
	label := i18n.T("shell.agentPluginGate.install")
	if installing {
		label = i18n.T("shell.agentPluginGate.installing")
	}
	install := on.Button(&g.install, label)
	install.Disabled = installing || len(picked) == 0
	d := th.Dialog(&g.dialog, len(offer) > 0, i18n.T("shell.agentPluginGate.title"), i18n.T("shell.agentPluginGate.description"))
	d.MaxWidth = optInWidth
	d.Body = func(gtx C) D {
		rows := make([]layout.FlexChild, len(offer))
		for i, p := range offer {
			rows[i] = layout.Rigid(func(gtx C) D { return g.offerRow(gtx, on, p, installing) })
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	}
	d.Actions = []layout.Widget{dontAsk.Layout, notNow.Layout, install.Layout}
	if d.Layout(gtx) && !installing {
		closeOffer()
	}
}

func (g *pluginGate) offerRow(gtx C, th *ui.Theme, p lichclient.PluginStatus, installing bool) D {
	click := g.rows[p.Provider]
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		if click.Hovered() && !installing {
			ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
		}
		return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
			box := th.Checkbox(g.picked[p.Provider])
			box.Disabled = installing
			children := []layout.FlexChild{
				layout.Rigid(box.Layout),
				ui.Gap(2.5),
				layout.Rigid(func(gtx C) D { return providerIcon(p.Provider).Layout(gtx, pluginProviderIcon, th.Foreground) }),
				ui.Gap(2.5),
				layout.Flexed(1, th.Text(ui.TextSM, p.Name).Weight(font.Medium).Layout),
			}
			if p.Provider == codexKind {
				children = append(children, layout.Rigid(th.Text(ui.TextXS, i18n.T("shell.agentPluginGate.needsHooks")).Muted().Layout))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	})
}
