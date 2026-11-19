package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

// settingsFrames is the Settings screen open on section, a "section/page"
// one opening that page, over a fake
// backend with Claude Code and Codex installed and on, framed the way the
// window frames it: its dialogs first, then the screen.
func settingsFrames(t *testing.T, section string) (*settingsScreen, *frameRig) {
	t.Helper()
	s, r, _ := settingsFramesOn(t, section)
	return s, r
}

// settingsFramesOn is settingsFrames with the fake backend it draws over, for
// a test that reads what a row stored.
func settingsFramesOn(t *testing.T, section string) (*settingsScreen, *frameRig, *sidebarBackend) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	f, client := newSidebarBackend(t)
	twoInstalled(f)
	root := new(ui.Root)
	s := newSettingsScreen(t.Context(), client, root, r.invalidate)
	s.providers.book = newModel(client, r.invalidate).providers
	if err := s.providers.book.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.theme = &themePicker{toaster: new(ui.Toaster)}
	s.notify = newNotifier(t.Context(), client, nil, r.invalidate)
	s.open, s.section, s.page = true, parseSettingsSection(section), settingsPageOf(section)
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layoutPatchNotesDialog(gtx, th)
			if s.open {
				s.layout(gtx, th, openTabs)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return s, r, f
}
