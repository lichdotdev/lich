package main

import (
	"flag"
	"strings"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var flagExtension = flag.String("extension", "", "open this extension surface at start, as id/surface, for -shot")

// extensionShot opens the surface -extension names.
func (u *window) extensionShot() {
	if id, surface, ok := strings.Cut(*flagExtension, "/"); ok {
		u.toggleExtension(extensionKey{id, surface})
	}
}

// openTabSurface opens the surface -tab names as ext:<extension>/<surface>,
// in the dock or the panel its manifest puts it in.
func (u *window) openTabSurface() {
	k, ok := extensionItemKey(*flagTab)
	if !ok {
		return
	}
	switch p, _ := findPlaced(placedSurfaces(u.model.extensions.all(), nil), k); p.place {
	case extension.PlaceDock:
		u.dock.toggleExtension(k)
	case extension.PlacePanel:
		u.extPanel.toggle(k)
	}
}

// toggleExtension is a surface's strip button: it opens the surface over
// Settings and Pulls, and closes it while it is open.
func (u *window) toggleExtension(k extensionKey) {
	if u.extension.isShown(k) {
		u.extension.hide()
	} else {
		u.settings.hide()
		u.pulls.hide()
		u.extension.show(k)
	}
	u.model.invalidate()
}

// applyExtension carries out the failed state's buttons, off the frame.
func (u *window) applyExtension(act extensionAction) {
	if id := act.restart; id != "" {
		go func() {
			if err := u.client.RestartExtension(u.ctx, id); err != nil {
				name := id
				if e, ok := u.model.extensions.byID(id); ok {
					name = extensionName(e)
				}
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.extensionScreen.restartFailed", i18n.Args{"name": name, "error": backend.ErrorText(err)})})
			}
		}()
	}
	if dir := act.folder; dir != "" {
		go func() {
			if err := u.client.OpenFolder(u.ctx, dir); err != nil {
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.openFolderFailed", i18n.Args{"error": backend.ErrorText(err)})})
			}
		}()
	}
}
