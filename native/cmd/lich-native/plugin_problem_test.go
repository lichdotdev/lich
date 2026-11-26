package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// pluginProblemFrames draws u's toasts, the way the window does.
func pluginProblemFrames(u *window) *frameRig {
	u.sidebar, u.dock, u.toastFloor = &sidebar{}, &dock{}, 60
	r := newFrameRig(rigWindow, nil)
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.layoutToasts(gtx)
	}
	// The toaster dates a dismissal by the clock, so the rig runs on it.
	r.now = time.Now()
	return r
}

// A failed pass is one notice: what lich could not do, the failure's own
// words and Retry; its end takes the notice down.
func TestPluginProblemIsOneNoticeUntilItEnds(t *testing.T) {
	u, _ := newGateWindow(t)
	r := pluginProblemFrames(u)
	const why = "cannot reach the lichdotdev/lich-plugin releases — are you online?"
	u.showPluginProblem(backend.PluginProblem{Bundle: true, Error: why})
	u.showPluginProblem(backend.PluginProblem{Bundle: true, Error: why})
	r.settle(t)
	if !r.showsAll(i18n.T("shell.pluginProblem.release"), why, i18n.T("shell.pluginProblem.retry")) {
		t.Fatalf("drawn: %s", r.drawn())
	}
	if n := len(u.toaster.Covers()); n != 1 {
		t.Errorf("%d notices, want one", n)
	}
	writeShot(t, r, "plugin-problem.png")

	u.showPluginProblem(backend.PluginProblem{})
	if u.toaster.Live(pluginProblemToast) {
		t.Error("the notice stayed after the failure ended")
	}
}

// A provider whose install failed is named in the notice.
func TestPluginProblemNamesTheProviders(t *testing.T) {
	u, _ := newGateWindow(t)
	r := pluginProblemFrames(u)
	u.showPluginProblem(backend.PluginProblem{Providers: []string{"Codex", "Kiro CLI"}, Error: "refused"})
	r.settle(t)
	if want := i18n.T("shell.pluginProblem.providers", i18n.Args{"names": "Codex, Kiro CLI"}); !r.shows(want) {
		t.Errorf("drawn: %s, want %q", r.drawn(), want)
	}
}

// Retry runs a pass and reads its outcome, which an unchanged failure sends no
// event for.
func TestPluginProblemRetryRunsAPassAndRereads(t *testing.T) {
	u, b := newGateWindow(t)
	r := pluginProblemFrames(u)
	u.showPluginProblem(backend.PluginProblem{Bundle: true, Error: "offline"})
	r.settle(t)
	r.clickLabel(t, i18n.T("shell.pluginProblem.retry"))
	b.await(t, "agentplugin.Retry")
	b.await(t, "agentplugin.Problem")
}
