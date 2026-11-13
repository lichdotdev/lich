package main

import (
	"strconv"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// extensionUpdateDismissedKey, followed by an extension's id, holds the
	// release Later was clicked for.
	extensionUpdateDismissedKey = "lich.extensionUpdateDismissed."
	extensionUpdateToastID      = "extension-update"
)

// extensionUpdateGate is the offer of the extensions' newer releases: one
// toast for all of them, raised when the backend finds one.
type extensionUpdateGate struct {
	mu sync.Mutex
	// settled is the release, by extension id, this run already answered:
	// put off, or sent to update.
	settled map[string]string
}

func (g *extensionUpdateGate) settle(list ...backend.Extension) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.settled == nil {
		g.settled = map[string]string{}
	}
	for _, e := range list {
		g.settled[e.ID] = e.Latest
	}
}

func (g *extensionUpdateGate) isSettled(e backend.Extension) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.settled[e.ID] == e.Latest
}

// pendingExtensionUpdates is the extensions with a newer release nobody
// answered, this run or with Later in an earlier one.
func (u *window) pendingExtensionUpdates() []backend.Extension {
	var pending []backend.Extension
	for _, e := range u.model.extensions.all() {
		if e.Latest == "" || u.extUpdates.isSettled(e) {
			continue
		}
		dismissed, err := u.client.GetSetting(u.ctx, extensionUpdateDismissedKey+e.ID, "")
		if err != nil {
			u.logErr("extension update", err)
			continue
		}
		if dismissed != e.Latest {
			pending = append(pending, e)
		}
	}
	return pending
}

// offerExtensionUpdates raises the toast for what is pending, replacing the
// one that is up, and takes it down once nothing is.
func (u *window) offerExtensionUpdates() {
	pending := u.pendingExtensionUpdates()
	if len(pending) == 0 {
		u.toaster.Dismiss(extensionUpdateToastID)
		return
	}
	u.toaster.Show(u.extensionUpdateToast(pending))
}

// extensionUpdateToast offers one extension's update in place, and several
// as the way to their rows in Settings.
func (u *window) extensionUpdateToast(pending []backend.Extension) ui.Toast {
	t := ui.Toast{ID: extensionUpdateToastID, Duration: ui.ToastForever}
	t.Cancel = &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.later"), OnClick: func() {
		u.extUpdates.settle(pending...)
		for _, e := range pending {
			u.writePref(extensionUpdateDismissedKey+e.ID, e.Latest)
		}
	}}
	if len(pending) > 1 {
		t.Title = i18n.T("shell.extensionUpdateGate.availableMany", i18n.Args{"n": strconv.Itoa(len(pending))})
		t.Action = &ui.ToastButton{Label: i18n.T("shell.extensionUpdateGate.review"), OnClick: func() {
			u.extUpdates.settle(pending...)
			u.showExtensionSettings()
		}}
		return t
	}
	e := pending[0]
	t.Title = i18n.T("shell.extensionUpdateGate.available", i18n.Args{"name": extensionName(e), "version": e.Latest})
	t.Secondary = &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.viewRelease"), OnClick: func() {
		u.settings.openExternal(e.Source + "/releases/tag/" + e.Latest)
	}}
	t.Action = &ui.ToastButton{Label: i18n.T("settings.extensionsSettings.update"), OnClick: func() {
		u.extUpdates.settle(e)
		u.settings.updateExtension(e)
	}}
	return t
}

// showExtensionSettings opens Settings on Extensions.
func (u *window) showExtensionSettings() {
	tabs, _ := u.model.projectTabs()
	u.settings.openSection(sectionExtensions)
	u.settings.show("", settingsProjectsOf(tabs))
}
