package main

import (
	"slices"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// updateExtension stages e's newer release and puts it in e's place: at once
// when it asks for nothing e does not already have, after the confirmation
// otherwise. Nothing happens while an install runs or waits for its answer.
func (s *settingsScreen) updateExtension(e backend.Extension) {
	p := &s.extensions
	start := false
	p.with(func(t *extensionTasks) {
		if t.installing == "" && !t.asking {
			start, t.installing = true, errAtRow(e.ID)
			if t.err.at == errAtRow(e.ID) {
				t.err = extensionError{}
			}
		}
	})
	if !start {
		return
	}
	go func() {
		preview, err := s.client.PrepareExtensionUpdate(s.ctx, e.ID)
		switch {
		case err != nil:
			s.updateFailed(e, err)
		case len(newPermissions(e.Permissions, preview.Permissions)) == 0:
			s.commitUpdate(e, preview)
		default:
			p.with(func(t *extensionTasks) {
				t.installing = ""
				t.asking, t.preview, t.askingFrom, t.replacing = true, preview, errAtRow(e.ID), &e
			})
			s.invalidate()
		}
	}()
}

// commitUpdate installs the staged update over e and says so in a toast,
// which is seen from any screen: an update also starts from the toast that
// offers it.
func (s *settingsScreen) commitUpdate(e backend.Extension, preview backend.ExtensionInstallPreview) {
	p := &s.extensions
	p.with(func(t *extensionTasks) { t.asking, t.installing = false, errAtRow(e.ID) })
	if _, err := s.client.CommitExtensionInstall(s.ctx, preview.Token); err != nil {
		s.updateFailed(e, err)
		return
	}
	p.with(func(t *extensionTasks) { t.installing = "" })
	// The new process's first status carries no newer release either, and may
	// be a moment away.
	p.store.setLatest(e.ID, "")
	s.theme.toaster.Show(ui.Toast{Variant: ui.ToastSuccess,
		Title: i18n.T("settings.extensionsSettings.updated", i18n.Args{"name": extensionName(e), "version": preview.Release})})
	s.invalidate()
}

// updateFailed puts the backend's words under e's row, and in a toast when
// Settings is not open to show that row: the update may have started from
// the toast that offered it.
func (s *settingsScreen) updateFailed(e backend.Extension, err error) {
	s.extensions.with(func(t *extensionTasks) {
		t.installing, t.err = "", extensionError{errAtRow(e.ID), backend.ErrorText(err)}
	})
	s.invalidate()
	if s.open {
		return
	}
	s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.extensionsSettings.updateFailed", i18n.Args{"name": extensionName(e)}), Description: backend.ErrorText(err)})
}

// checkExtensionUpdates is Check for updates: the rows take what it found,
// and a toast answers when that is nothing.
func (s *settingsScreen) checkExtensionUpdates() {
	p := &s.extensions
	start := false
	p.with(func(t *extensionTasks) {
		if !t.checking {
			start, t.checking = true, true
		}
	})
	if !start {
		return
	}
	go func() {
		list, err := s.client.CheckExtensionUpdates(s.ctx)
		p.with(func(t *extensionTasks) { t.checking = false })
		for _, e := range list {
			p.store.setLatest(e.ID, e.Latest)
		}
		switch {
		case err != nil:
			s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("settings.extensionsSettings.checkFailed"), Description: backend.ErrorText(err)})
		case !slices.ContainsFunc(list, func(e backend.Extension) bool { return e.Latest != "" }):
			s.theme.toaster.Show(ui.Toast{Variant: ui.ToastSuccess, Title: i18n.T("settings.extensionsSettings.upToDate")})
		}
		s.invalidate()
	}()
}
