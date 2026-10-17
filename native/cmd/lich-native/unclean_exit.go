package main

import (
	"log"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// uncleanExitToast is UncleanExitGate's warning: it stays until dismissed,
// and its action opens the log's folder.
func uncleanExitToast(openLog func()) ui.Toast {
	return ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("shell.uncleanExitGate.title"), Description: i18n.T("shell.uncleanExitGate.description"),
		Action: &ui.ToastButton{Label: i18n.T("shell.uncleanExitGate.openLogFolder"), OnClick: openLog}, Cancel: &ui.ToastButton{Label: i18n.T("shell.uncleanExitGate.dismiss")},
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
