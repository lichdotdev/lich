package main

import (
	"log"
	"strings"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// pluginProblemToast names the one notice of what lich could not do to give its
// sessions their plugin: each new answer replaces it, and the end of the
// failure takes it down.
const pluginProblemToast = "lich-plugin-problem"

// checkPluginProblem reads the last pass's outcome once at start; later ones
// arrive on api.PluginProblemEvent.
func (u *window) checkPluginProblem() {
	p, err := u.client.PluginProblem(u.ctx)
	if err != nil {
		log.Printf("plugin problem: %v", err)
		return
	}
	u.showPluginProblem(p)
}

// showPluginProblem raises the notice, with the failure's own text under it
// and Retry, or takes it down when nothing failed.
func (u *window) showPluginProblem(p backend.PluginProblem) {
	if p.Error == "" {
		u.toaster.Dismiss(pluginProblemToast)
		return
	}
	title := i18n.T("shell.pluginProblem.release")
	if !p.Bundle {
		title = i18n.T("shell.pluginProblem.providers", i18n.Args{"names": strings.Join(p.Providers, ", ")})
	}
	u.toaster.Show(ui.Toast{ID: pluginProblemToast, Variant: ui.ToastWarning, Title: title, Description: p.Error, Duration: ui.ToastForever,
		Action: &ui.ToastButton{Label: i18n.T("shell.pluginProblem.retry"), OnClick: func() { go u.retryPlugin() }}})
}

// retryPlugin runs a pass and shows its outcome, which an unchanged failure
// sends no event for.
func (u *window) retryPlugin() {
	if err := u.client.RetryPlugin(u.ctx); err != nil {
		log.Printf("plugin retry: %v", err)
	}
	u.checkPluginProblem()
}
