package main

import (
	"log"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n; the web's en catalog.
const (
	uncleanExitTitle   = "lich's previous run ended unexpectedly"
	uncleanExitDesc    = "Your sessions were restored, but whatever they were running stopped with it."
	uncleanExitOpenLog = "Open log folder"
	uncleanExitDismiss = "Dismiss"
)

// uncleanExitToast is UncleanExitGate's warning: it stays until dismissed,
// and its action opens the log's folder.
func uncleanExitToast(openLog func()) ui.Toast {
	return ui.Toast{Variant: ui.ToastWarning, Title: uncleanExitTitle, Description: uncleanExitDesc,
		Action: &ui.ToastButton{Label: uncleanExitOpenLog, OnClick: openLog}, Cancel: &ui.ToastButton{Label: uncleanExitDismiss},
		Duration: ui.ToastForever}
}

// warnUncleanExit is UncleanExitGate: one ask at start.
func (u *window) warnUncleanExit() {
	unclean, err := u.client.TakeUncleanExit(u.ctx)
	if err != nil {
		log.Printf("unclean exit: %v", err)
		return
	}
	if unclean {
		u.toaster.Show(uncleanExitToast(u.settings.revealLog))
	}
}
