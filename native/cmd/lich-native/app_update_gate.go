package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (shell.appUpdateGate,
// shell.updateProgressToast, update.progress).
const (
	appUpdateAndInstall    = "Update & install"
	appUpdateLater         = "Later"
	appUpdateInstall       = "Install"
	appUpdateUpdated       = "lich updated to %s"
	appUpdateRetry         = "Retry"
	appUpdateRestart       = "Restart"
	appUpdateRestartFailed = "Restart failed: %v"
	appUpdateDownloading   = "Downloading lich %s"
	appUpdateInstalling    = "Installing lich %s…"
	appUpdateReopens       = "lich closes and reopens on its own."
	appUpdateFailed        = "Update failed: %s"
	appUpdateInstallFailed = "Install failed: %s"
	appUpdateDownloadFail  = "Download failed: %s"
	appUpdateDownloadAt    = "Download failed at %d%%: %s"
	appUpdateBytesOf       = "%s of %s"
	appUpdatePercentBytes  = "%d%% · %s"
)

const (
	// appUpdateDismissedKey is app-update-gate.ts's UPDATE_DISMISSED_KEY:
	// the version Later was clicked for.
	appUpdateDismissedKey = "lich.appUpdateDismissed"
	// appUpdatePollEvery is the web's POLL_INTERVAL_MS: releases are rare, and
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
	last     *lichclient.AppUpdateProgress
}

// shouldPromptUpdate is decideUpdateAction: a newer release Later was not
// clicked for.
func shouldPromptUpdate(st lichclient.AppUpdateStatus, dismissed string) bool {
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

// promptAppUpdate offers the installer where lich may write its own
// install, and otherwise the command to paste and the release page.
func (u *window) promptAppUpdate(st lichclient.AppUpdateStatus) {
	g := &u.appUpdate
	g.mu.Lock()
	g.prompted = st.LatestVersion
	g.mu.Unlock()
	version := st.LatestVersion
	later := &ui.ToastButton{Label: appUpdateLater, OnClick: func() { u.writePref(appUpdateDismissedKey, version) }}
	t := ui.Toast{ID: appUpdateToastID, Title: fmt.Sprintf(updatesToastTitle, version), Duration: ui.ToastForever, Cancel: later}
	release := &ui.ToastButton{Label: updatesViewRelease, OnClick: func() { u.settings.openExternal(st.ReleaseURL) }}
	switch {
	case st.CanSelfApply:
		t.Action = &ui.ToastButton{Label: appUpdateAndInstall, OnClick: func() { go u.applyUpdate(version) }}
	case st.InstallCommand != "":
		t.Action = &ui.ToastButton{Label: appUpdateInstall, OnClick: func() { u.openInstall(st.ReleaseURL, st.InstallCommand) }}
		t.Secondary = release
	default:
		t.Action = release
	}
	u.toaster.Show(t)
}

// openInstall opens a shell in the project in view and types the install
// command there for the user to run; with no project open it opens the
// release page instead.
// ponytail: the web opens a $HOME project when none is open; the native
// window has no such project yet, so the release page stands in.
func (u *window) openInstall(releaseURL, command string) {
	if _, ok := u.model.project(); !ok {
		u.settings.openExternal(releaseURL)
		return
	}
	u.settings.hide()
	go u.newSession(launchRequest{kind: "shell"}, command)
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
			Title:  updateFailureText(last, lichclient.ErrorText(err)),
			Action: &ui.ToastButton{Label: appUpdateRetry, OnClick: func() { go u.applyUpdate(version) }}})
		return
	}
	u.toaster.Show(ui.Toast{ID: appUpdateToastID, Variant: ui.ToastSuccess, Duration: ui.ToastForever,
		Title: fmt.Sprintf(appUpdateUpdated, version),
		Action: &ui.ToastButton{Label: appUpdateRestart, OnClick: func() {
			go func() {
				if err := u.client.Restart(u.ctx); err != nil {
					u.toaster.Show(failure(appUpdateRestartFailed, err))
				}
			}()
		}}})
}

// updateProgressed is the progress event: it redraws the toast while an
// apply runs, and is dropped otherwise.
func (u *window) updateProgressed(p lichclient.AppUpdateProgress) {
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

// updateProgressToast is UpdateProgressToast as a toast's title and
// description: the percent and bytes stand in for the web's bar.
func updateProgressToast(version string, p *lichclient.AppUpdateProgress) ui.Toast {
	t := ui.Toast{ID: appUpdateToastID, Duration: ui.ToastForever, Title: fmt.Sprintf(appUpdateDownloading, version)}
	switch {
	case p == nil:
	case p.Phase == lichclient.UpdatePhaseInstaller:
		t.Title, t.Description = fmt.Sprintf(appUpdateInstalling, version), appUpdateReopens
	default:
		percent, bytes := downloadReading(*p)
		t.Description = bytes
		if percent >= 0 {
			t.Description = fmt.Sprintf(appUpdatePercentBytes, percent, bytes)
		}
	}
	return t
}

// downloadReading is update-progress.ts's: percent is -1 while the total is
// unknown.
func downloadReading(p lichclient.AppUpdateProgress) (percent int, bytes string) {
	if p.Total <= 0 {
		return -1, megabytes(p.Received)
	}
	return int(min(100, p.Received*100/p.Total)), fmt.Sprintf(appUpdateBytesOf, megabytes(p.Received), megabytes(p.Total))
}

func megabytes(n int64) string { return fmt.Sprintf("%.1f MB", float64(n)/bytesPerMB) }

// updateFailureText names the phase the update died in and, for a download,
// how far it got.
func updateFailureText(last *lichclient.AppUpdateProgress, err string) string {
	switch {
	case last == nil:
		return fmt.Sprintf(appUpdateFailed, err)
	case last.Phase != lichclient.UpdatePhaseDownload:
		return fmt.Sprintf(appUpdateInstallFailed, err)
	}
	if percent, _ := downloadReading(*last); percent >= 0 {
		return fmt.Sprintf(appUpdateDownloadAt, percent, err)
	}
	return fmt.Sprintf(appUpdateDownloadFail, err)
}
