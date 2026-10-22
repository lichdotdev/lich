package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

// settingsFrames is the Settings screen open on section, over a fake
// backend, framed the way the window frames it: its dialogs first, then the
// screen.
func settingsFrames(t *testing.T, section string) (*settingsScreen, *frameRig) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	_, client := newSidebarBackend(t)
	root := new(ui.Root)
	s := newSettingsScreen(t.Context(), client, root, r.invalidate)
	s.theme = &themePicker{toaster: new(ui.Toaster)}
	s.notify = newNotifier(t.Context(), client, nil, r.invalidate)
	s.open, s.section = true, section
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layoutPatchNotesDialog(gtx, th)
			if s.open {
				s.layout(gtx, th, nil)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return s, r
}
