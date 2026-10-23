package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const (
	// appUpdateDismissedKey holds the version Later was clicked for.
	appUpdateDismissedKey = "lich.appUpdateDismissed"
	// appUpdatePollEvery is the poll interval: releases are rare, and
	// the unauthenticated GitHub API allows 60 requests an hour per IP.
	appUpdatePollEvery = time.Hour
	appUpdateToastID   = "app-update"
	bytesPerMB         = 1_000_000
)

// appUpdateGate is AppUpdateGate: a check at start and hourly, a prompt per
// release this run, and the Windows installer's progress.
type appUpdateGate struct {
	mu       sync.Mutex
	prompted string
	// applying is the version ApplyUpdate is installing, "" when idle; last
	// is its latest step, nil before the first.
	applying string
	last     *backend.AppUpdateProgress
}

// shouldPromptUpdate is decideUpdateAction: a newer release Later was not
// clicked for.
func shouldPromptUpdate(st backend.AppUpdateStatus, dismissed string) bool {
	return st.UpdateAvailable && dismissed != st.LatestVersion
}

func (u *window) pollAppUpdate() {
	tick := time.NewTicker(appUpdatePollEvery)
	defer tick.Stop()
	for {
		u.checkAppUpdate()
		select {
		case <-u.ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (u *window) checkAppUpdate() {
	st, err := u.client.AppUpdateStatus(u.ctx)
	if err != nil {
		log.Printf("app update: %v", err)
		return
	}
	dismissed, err := u.client.GetSetting(u.ctx, appUpdateDismissedKey, "")
	if err != nil {
		log.Printf("app update: %v", err)
		return
	}
	if !shouldPromptUpdate(st, dismissed) {
		return
	}
	g := &u.appUpdate
	g.mu.Lock()
	seen := g.prompted == st.LatestVersion
	g.mu.Unlock()
	if !seen {
		u.promptAppUpdate(st)
	}
}

// promptAppUpdate raises st's prompt, once per release this run.
func (u *window) promptAppUpdate(st backend.AppUpdateStatus) {
	g := &u.appUpdate
	g.mu.Lock()
	g.prompted = st.LatestVersion
	g.mu.Unlock()
	u.toaster.Show(u.appUpdateToast(st))
}

// appUpdateToast offers the installer where lich may write its own install,
// and otherwise the command to paste and the release page.
func (u *window) appUpdateToast(st backend.AppUpdateStatus) ui.Toast {
	version := st.LatestVersion
	later := &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.later"), OnClick: func() { u.writePref(appUpdateDismissedKey, version) }}
	t := ui.Toast{ID: appUpdateToastID, Title: i18n.T("shell.appUpdateGate.available", i18n.Args{"version": version}), Duration: ui.ToastForever, Cancel: later}
	release := &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.viewRelease"), OnClick: func() { u.settings.openExternal(st.ReleaseURL) }}
	switch {
	case st.CanSelfApply:
		t.Action = &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.updateAndInstall"), OnClick: func() { go u.applyUpdate(version) }}
	case st.InstallCommand != "":
		t.Action = &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.install"), OnClick: func() { u.openInstall(st.ReleaseURL, st.InstallCommand) }}
		t.Secondary = release
	default:
		t.Action = release
	}
	return t
}

// openInstall opens a shell in the project in view and types the install
// command there for the user to run. With no project open it opens Home's
// first, and the release page only when even that fails.
func (u *window) openInstall(releaseURL, command string) {
	u.settings.hide()
	go func() {
		if _, ok := u.model.project(); !ok {
			if err := u.model.openHome(u.ctx); err != nil {
				log.Printf("install: open the home project: %v", err)
				u.settings.openExternal(releaseURL)
				return
			}
		}
		u.newSession(launchRequest{kind: "shell"}, command)
	}()
}

// applyUpdate is runApply: the installer's progress in one toast, replaced
// in place, ending on Restart or on Retry.
func (u *window) applyUpdate(version string) {
	g := &u.appUpdate
	g.mu.Lock()
	g.applying, g.last = version, nil
	g.mu.Unlock()
	u.toaster.Show(updateProgressToast(version, nil))
	err := u.client.ApplyUpdate(u.ctx)
	g.mu.Lock()
	last := g.last
	g.applying = ""
	g.mu.Unlock()
	if err != nil {
		u.toaster.Show(ui.Toast{ID: appUpdateToastID, Variant: ui.ToastError, Duration: ui.ToastForever,
			Title:  updateFailureText(last, backend.ErrorText(err)),
			Action: &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.retry"), OnClick: func() { go u.applyUpdate(version) }}})
		return
	}
	u.toaster.Show(ui.Toast{ID: appUpdateToastID, Variant: ui.ToastSuccess, Duration: ui.ToastForever,
		Title: i18n.T("shell.appUpdateGate.updated", i18n.Args{"version": version}),
		Action: &ui.ToastButton{Label: i18n.T("shell.appUpdateGate.restart"), OnClick: func() {
			go func() {
				if err := u.client.Restart(u.ctx); err != nil {
					u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.appUpdateGate.restartFailed", i18n.Args{"error": backend.ErrorText(err)})})
				}
			}()
		}}})
}

// updateProgressed is the progress event: it redraws the toast while an
// apply runs, and is dropped otherwise.
func (u *window) updateProgressed(p backend.AppUpdateProgress) {
	g := &u.appUpdate
	g.mu.Lock()
	version := g.applying
	if version != "" {
		g.last = &p
	}
	g.mu.Unlock()
	if version != "" {
		u.toaster.Show(updateProgressToast(version, &p))
	}
}

// updateProgressToast is UpdateProgressToast: the download's bar, with its
// percent and bytes under it, then the installer's step behind a spinner.
func updateProgressToast(version string, p *backend.AppUpdateProgress) ui.Toast {
	t := ui.Toast{ID: appUpdateToastID, Duration: ui.ToastForever, Title: i18n.T("shell.updateProgressToast.downloading", i18n.Args{"version": version}),
		Progress: &ui.ToastProgress{Percent: -1}}
	switch {
	case p == nil:
	case p.Phase == backend.UpdatePhaseInstaller:
		t.Title, t.Description, t.Progress = i18n.T("shell.updateProgressToast.installing", i18n.Args{"version": version}), i18n.T("shell.updateProgressToast.reopens"), nil
		t.Spin = true
	default:
		percent, bytes := downloadReading(*p)
		t.Description, t.Progress.Percent = bytes, percent
		if percent >= 0 {
			t.Description = i18n.T("update.appUpdateGate.percentBytes", i18n.Args{"percent": percent, "bytes": bytes})
		}
	}
	return t
}

// downloadReading reads a download's progress: percent is -1 while the total is
// unknown.
func downloadReading(p backend.AppUpdateProgress) (percent int, bytes string) {
	if p.Total <= 0 {
		return -1, megabytes(p.Received)
	}
	return int(min(100, p.Received*100/p.Total)), i18n.T("update.progress.bytesOf", i18n.Args{"received": megabytes(p.Received), "total": megabytes(p.Total)})
}

func megabytes(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/bytesPerMB) }

// updateFailureText names the phase the update died in and, for a download,
// how far it got.
func updateFailureText(last *backend.AppUpdateProgress, err string) string {
	switch {
	case last == nil:
		return i18n.T("update.progress.updateFailed", i18n.Args{"error": err})
	case last.Phase != backend.UpdatePhaseDownload:
		return i18n.T("update.progress.installFailed", i18n.Args{"error": err})
	}
	if percent, _ := downloadReading(*last); percent >= 0 {
		return i18n.T("update.progress.downloadFailedAt", i18n.Args{"percent": percent, "error": err})
	}
	return i18n.T("update.progress.downloadFailed", i18n.Args{"error": err})
}
