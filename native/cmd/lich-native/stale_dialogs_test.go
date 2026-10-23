package main

import (
	"image"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// rigWindow is a desktop window, room for every dialog.
var rigWindow = image.Pt(1200, 800)

func TestClosingAProjectWithAnAgentRunningAsksInTheFrameAfterTheClick(t *testing.T) {
	u := &window{}
	r := newFrameRig(rigWindow, nil)
	u.model = newModel(nil, r.invalidate)
	u.model.projects = threeProjects()
	u.model.setActiveProject("b")
	report(t, u.model, "b1", statusBusy)
	u.topbar = newTopbar(&u.root)
	r.draw = func(gtx C, th *ui.Theme) {
		u.root.Layout(gtx, func(gtx C) D {
			closed := u.topbar.layoutConfirm(gtx, th)
			tabs, active := u.model.stripTabs()
			top := u.topbar.layout(gtx, th, topbarFrame{tabs: tabs, active: active})
			top.close = closed
			u.applyTopbar(top)
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.settle(t)
	tab := u.topbar.laid[1].rect
	_, next := r.click(t, r.smallestClickIn(t, tab))
	if !strings.Contains(next, i18n.T("tabs.projectTabs.closeAnyway")) {
		t.Errorf("the close confirmation never came up:\n%s", next)
	}
}

func TestLayingOutDelegatesFromOtherLayoutsAsksInTheFrameAfterTheClick(t *testing.T) {
	r := newFrameRig(rigWindow, nil)
	m, _ := newDelegatesModel(t)
	m.invalidate = r.invalidate
	m.splitBeside(t.Context(), "notes", "wa", edgeRight, false)
	m.activeProject, m.active = "lich", "orch"
	u := &window{ctx: t.Context(), model: m, footer: newFooterRig(t, nil).f}
	u.layoutStage.pointer = dragPointer{last: map[string]image.Rectangle{"orch": image.Rect(0, 0, 1000, 600)}, paneW: 480, paneH: 180, gap: 6}
	u.footer.showDelegates = func(id string) { u.layoutStage.delegates = id }
	u.footer.dock = &dock{}
	in := busySession()
	in.sessionID, in.delegates = "orch", 2
	r.draw = func(gtx C, th *ui.Theme) {
		u.root.Layout(gtx, func(gtx C) D {
			u.layoutMoveConfirm(gtx, th)
			u.footer.layoutBar(gtx, th, in)
			u.applyLayoutStage()
			return D{Size: gtx.Constraints.Max}
		})
	}
	_, next := r.clickLabel(t, i18n.T("layouts.delegates.showBeside", i18n.Args{"count": 2}))
	if !strings.Contains(next, i18n.T("layouts.move.confirm")) {
		t.Errorf("the move question never came up:\n%s", next)
	}
}

func TestViewPatchNotesOpensThemInTheFrameAfterTheClick(t *testing.T) {
	s, r := settingsFrames(t, sectionUpdates)
	s.updates.loaded = true
	s.updates.notes = &backend.PatchNotes{Version: "1.4.0", Groups: []backend.PatchNotesGroup{}}
	_, next := r.clickLabel(t, i18n.T("settings.updatesSettings.viewPatchNotes"))
	if !strings.Contains(next, i18n.T("shell.patchNotesDialog.gotIt")) {
		t.Errorf("the patch notes never came up:\n%s", next)
	}
}

func pluginGateFrames(t *testing.T, offer ...backend.PluginStatus) (*window, *frameRig) {
	t.Helper()
	u, _ := newGateWindow(t)
	u.plugin.started, u.plugin.offer = true, offer
	r := newFrameRig(rigWindow, func(gtx C, th *ui.Theme) {
		u.root.Layout(gtx, func(gtx C) D {
			u.layoutPluginGate(gtx, th)
			return D{Size: gtx.Constraints.Max}
		})
	})
	return u, r
}

func TestPluginGateClosesInTheFrameAfterTheClick(t *testing.T) {
	for _, button := range []string{"shell.agentPluginGate.notNow", "shell.agentPluginGate.dontAskAgain"} {
		t.Run(button, func(t *testing.T) {
			_, r := pluginGateFrames(t, backend.PluginStatus{Provider: "claude", Name: "Claude Code"})
			_, next := r.clickLabel(t, i18n.T(button))
			if strings.Contains(r.settle(t), i18n.T(button)) {
				t.Errorf("the gate stayed up:\n%s", next)
			}
		})
	}
}

func TestPluginGateDisablesInstallInTheFrameThatUntickedTheLastCLI(t *testing.T) {
	_, r := pluginGateFrames(t, backend.PluginStatus{Provider: "claude", Name: "Claude Code"})
	r.settle(t)
	label := r.at(t, "Claude Code")
	box := r.smallestClickIn(t, image.Rect(0, int(label.Y)-20, int(label.X), int(label.Y)+20))
	r.click(t, box)
}
