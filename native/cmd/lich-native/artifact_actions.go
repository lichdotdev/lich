package main

import (
	"context"
	"runtime"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func (u *window) newArtifactsTab() *artifactsTab {
	t := newArtifactsTab(&u.model.artifacts)
	t.peers = u.model.projectPeers
	t.open = u.openArtifact
	t.attach = u.attachArtifact
	t.reveal = u.revealFolder
	return t
}

// openArtifact loads artifact name of session from in session's browser,
// starting it when the session has none, and puts the browser on screen.
func (u *window) openArtifact(session, from, name string) {
	u.sendBrowser(session, "open", func(ctx context.Context) error { return u.client.OpenArtifact(ctx, session, from, name) })
	u.openBrowser(session)
}

// attachArtifact pastes, unsent, at target's prompt the path the backend
// readies for artifact name of session from, as the footer's attach does.
func (u *window) attachArtifact(target, from, name string) {
	go func() {
		a, err := u.client.AttachArtifact(u.ctx, target, from, name)
		if err == nil && a.Path != "" {
			err = u.terms.writePaste(u.ctx, target, composeDroppedPaths([]string{a.Path}, runtime.GOOS == "windows", []string{a.Notice}))
		}
		if err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: backend.ErrorText(err)})
		}
	}()
}

// revealFolder opens dir in the system's file manager, as the card's
// Open in › File manager does.
func (u *window) revealFolder(dir string) {
	go func() {
		if err := u.client.OpenFolder(u.ctx, dir); err != nil {
			u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.openFolderFailed", i18n.Args{"error": backend.ErrorText(err)})})
		}
	}()
}
