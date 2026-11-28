package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func TestServiceDialogKeepsFocusFromTheTerminal(t *testing.T) {
	terminalTag := new(int)
	u := window{
		sidebar:  newSidebar(nil),
		footer:   &footer{},
		dock:     &dock{tab: dockClosed},
		git:      &gitPanel{},
		topbar:   &topbar{},
		settings: &settingsScreen{},
		palette:  &commandPalette{},
		worktree: &worktreeDialog{},
	}
	rig := newFrameRig(image.Pt(1000, 800), func(gtx C, th *ui.Theme) {
		u.settings.layoutServiceDialog(gtx, th)
		if u.terminalTakesKeys(gtx) {
			gtx.Execute(key.FocusCmd{Tag: terminalTag})
		}
	})
	rig.frame()
	u.settings.services.begin(backend.Service{ProjectID: "p1"}, "Project")
	for range 3 {
		rig.frame()
		rig.now = rig.now.Add(50 * time.Millisecond)
	}
	if !rig.router.Source().Focused(&u.settings.services.name) {
		t.Fatal("the service name field lost focus to the terminal")
	}
}
