package main

import (
	"image"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

func TestTheStripTopsOnlyALoneAgentSession(t *testing.T) {
	u := &window{settings: new(settingsScreen)}
	agent := sessionView{ID: "a", Kind: "claude"}
	chat := sessionView{ID: "c", Kind: "claude",
		View: api.ViewChat}
	codex := sessionView{ID: "x", Kind: "codex"}
	shell := sessionView{ID: "s", Kind: "shell"}
	cases := []struct {
		name string
		v    stageView
		want int
	}{
		{"an agent's terminal", stageView{active: agent, hasActive: true}, 44},
		{"an agent's chat", stageView{active: chat, hasActive: true}, 44},
		{"a provider with no chat", stageView{active: codex, hasActive: true}, 44},
		{"a plain terminal", stageView{active: shell, hasActive: true}, 0},
		{"a split, whose panes have a header", stageView{hasLayout: true, active: agent, hasActive: true, panes: map[string]sessionView{"a": agent}}, 0},
		{"a service's console", stageView{active: agent, hasActive: true, hasConsole: true}, 0},
		{"Home", stageView{home: true}, 0},
		{"a project with no session", stageView{}, 0},
	}
	for _, c := range cases {
		if got := u.sessionBand(pxGtx(), c.v); got != c.want {
			t.Errorf("%s: strip %d, want %d", c.name, got, c.want)
		}
	}
}

func TestTheGitPanelLeavesTheStripAlone(t *testing.T) {
	_, client := newSidebarBackend(t)
	u := &window{settings: new(settingsScreen)}
	u.git = newGitPanel(t.Context(), client, &u.root, func() {}, func(ui.Toast) {}, "monospace", 12)
	u.git.open = true
	v := stageView{active: sessionView{ID: "a", Kind: "claude"}, hasActive: true, in: busySession()}
	if got := u.sessionBand(pxGtx(), v); got != 44 {
		t.Errorf("strip %d under an open Git panel, want 44: the grid is not resized from the top", got)
	}
}

// stripFrames is session s1 drawn under its strip, w wide, named label.
func stripFrames(t *testing.T, w int, label string) (*window, *frameRig) {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewTerminal)
	r.size = image.Pt(w, 400)
	u.model.projects[0].Sessions[0].Label = label
	u.model.providers = &providerBook{machine: providersMachine{list: []providerInfo{{id: "claude", name: "Claude Code"}}}}
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	r.frame()
	return u, r
}

// selectorLeft is the left edge of the selector's first item, the control
// under the GUI label, and ok when it is drawn.
func selectorLeft(t *testing.T, r *frameRig) (int, bool) {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == i18n.T("chat.switch.gui") {
			b, ok := r.smallestClickAt(n.Desc.Bounds.Min.Add(image.Pt(2, 2)))
			return b.Min.X, ok
		}
	}
	return 0, false
}

func TestTheStripNamesTheProviderAndTheSessionAtTheLeft(t *testing.T) {
	_, r := stripFrames(t, 800, "Cart totals a cent off")
	var name image.Rectangle
	for _, n := range r.nodes() {
		if n.Desc.Label == "Cart totals a cent off" {
			name = n.Desc.Bounds
		}
	}
	left, ok := selectorLeft(t, r)
	if name.Empty() || !ok {
		t.Fatalf("the strip lacks a part:\n%s", r.drawn())
	}
	if name.Min.X <= int(balloonInset) || name.Min.X >= left {
		t.Errorf("name at %d, selector at %d: want the island's edge, then the provider's mark, the name, the selector", name.Min.X, left)
	}
}

func TestALongNameIsCutBeforeTheSelector(t *testing.T) {
	_, r := stripFrames(t, 360, strings.Repeat("a very long session name ", 12))
	left, ok := selectorLeft(t, r)
	if !ok {
		t.Fatalf("the selector was pushed out:\n%s", r.drawn())
	}
	for _, n := range r.nodes() {
		if strings.HasPrefix(n.Desc.Label, "a very long") && n.Desc.Bounds.Max.X > left {
			t.Errorf("the name reaches %d, over the selector at %d", n.Desc.Bounds.Max.X, left)
		}
	}
	if left+selectorWidthAtLeast > 360 {
		t.Errorf("the selector at %d runs off a 360 strip", left)
	}
}

// selectorWidthAtLeast is a floor for the two items: below it the selector
// was cut, which it is never to be.
const selectorWidthAtLeast = 60

func TestTheSelectorClicksFromTheStrip(t *testing.T) {
	u, r := stripFrames(t, 800, "Cart totals a cent off")
	at := r.at(t, i18n.T("chat.switch.gui"))
	if int(at.Y) >= int(sessionBandHeight) {
		t.Fatalf("the selector is at y=%v, not in the strip", at.Y)
	}
	r.click(t, at)
	if !u.model.chat.isSwitching("s1") {
		t.Error("the click in the strip did not switch")
	}
}

func TestASplitPaneHasTheSelectorInItsHeaderAndNoStrip(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewTerminal)
	sv, _ := u.model.sessionViewOf("s1")
	sv.Label = "Cart totals a cent off"
	v := stageView{hasLayout: true, layout: layoutTab{Focus: "s1"}, panes: map[string]sessionView{"s1": sv}}
	u.sidebar, u.footer, u.dock, u.git, u.extPanel, u.topbar = &sidebar{}, &footer{}, &dock{}, &gitPanel{}, &extensionPanel{}, &topbar{}
	u.palette, u.worktree = &commandPalette{}, &worktreeDialog{}
	u.terms = &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}
	u.gate = &spawnGate{probe: &fakeProbe{live: true}, invalidate: r.invalidate, deciding: "another"}
	pane := image.Rect(0, 0, 600, 300)
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			if err := u.paneAt(gtx, v, "s1", pane, false); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.frame()
	at := r.at(t, i18n.T("chat.switch.gui"))
	if int(at.Y) >= int(paneHeader) {
		t.Errorf("the selector is at y=%v, below the %v header", at.Y, paneHeader)
	}
	if got := u.sessionBand(pxGtx(), v); got != 0 {
		t.Errorf("a split's pane lost %d more rows at its top", got)
	}
	r.click(t, at)
	if !u.model.chat.isSwitching("s1") {
		t.Error("the click in the header did not switch")
	}
}
