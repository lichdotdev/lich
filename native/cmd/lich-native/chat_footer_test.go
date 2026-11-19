package main

import (
	"image"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestAChatCarriesNoBalloonsAndNoBand(t *testing.T) {
	u := &window{settings: new(settingsScreen)}
	chat := sessionView{ID: "a", Kind: "claude",
		View: api.ViewChat}
	terminal := sessionView{ID: "t", Kind: "claude"}
	cases := []struct {
		name string
		v    stageView
		want int
	}{
		{"a chat", stageView{active: chat, hasActive: true}, 0},
		{"a layout whose focus is a chat", stageView{active: chat, hasActive: true, hasLayout: true, panes: map[string]sessionView{"a": chat, "t": terminal}}, 0},
		{"a layout whose focus is a terminal", stageView{active: terminal, hasActive: true, hasLayout: true, panes: map[string]sessionView{"a": chat, "t": terminal}}, 52},
		{"the same agent as a terminal", stageView{active: terminal, hasActive: true}, 52},
	}
	for _, c := range cases {
		if got := u.ptyBand(pxGtx(), c.v); got != c.want {
			t.Errorf("%s: band %d, want %d", c.name, got, c.want)
		}
		if got := u.balloonsShown(c.v); got != (c.want > 0) {
			t.Errorf("%s: balloons shown %v with a band of %d", c.name, got, c.want)
		}
	}
}

// y of a control's centre, for checking which line it stands on.
func centreY(t *testing.T, r *frameRig, label string) float32 {
	t.Helper()
	return r.at(t, label).Y
}

func TestTheComposerCarriesTheFootersChipsAndSendOnOneLine(t *testing.T) {
	_, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	line := centreY(t, r, "PR #7")
	prev := r.at(t, "PR #7").X
	for _, label := range []string{"42%", "5h 28%", "10m", i18n.T("chat.composer.send")} {
		p := r.at(t, label)
		if p.Y < line-8 || p.Y > line+8 {
			t.Errorf("%q at y=%v, want the chips' line (y=%v)", label, p.Y, line)
		}
		if p.X <= prev {
			t.Errorf("%q at x=%v, want it after the item before it (x=%v): the left side, the right side, then Send", label, p.X, prev)
		}
		prev = p.X
	}
	if branch := centreY(t, r, "main"); branch <= line+8 {
		t.Errorf("the branch at y=%v, want it on its own line under the box (chips at y=%v)", branch, line)
	}
	if r.shows("Settings") {
		t.Error("the composer drew something past the footer's items")
	}
}

func labelLeft(t *testing.T, r *frameRig, label string) int {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			return n.Desc.Bounds.Min.X
		}
	}
	return -1
}

func TestTheBranchUnderTheComposerOpensTheGitPanel(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	if u.footer.git.open {
		t.Fatal("the Git panel starts open")
	}
	r.click(t, r.at(t, "main"))
	if !u.footer.git.open {
		t.Error("the branch did not open the Git panel")
	}
}

func TestTheProjectLeadsTheBranchUnderTheComposer(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.projects[0].Name = "lich"
	r.settle(t)
	if !r.shows("lich") || !r.shows("/") {
		t.Fatalf("no project and slash before the branch:\n%s", r.drawn())
	}
	if project, branch := labelTop(t, r, "lich"), labelTop(t, r, "main"); project != branch {
		t.Errorf("project's top at y=%d, branch's at y=%d: want them on one line", project, branch)
	}
	if project, branch := labelLeft(t, r, "lich"), labelLeft(t, r, "main"); project >= branch {
		t.Errorf("project at x=%d, branch at x=%d: want the project first", project, branch)
	}
}

func TestAStopTakesSendsPlaceWhileTheAgentWorks(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	send := r.at(t, i18n.T("chat.composer.send"))
	u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	r.settle(t)
	stop := r.at(t, i18n.T("chat.composer.stop"))
	if r.shows(i18n.T("chat.composer.send")) {
		t.Error("Send stayed while the agent worked")
	}
	if d := stop.Y - send.Y; d < -8 || d > 8 || stop.X < send.X-40 {
		t.Errorf("Stop at %v, Send was at %v: want it in the same corner of the box", stop, send)
	}
}

func TestANarrowComposerLeavesOutWhatDoesNotFitRatherThanWrap(t *testing.T) {
	_, r, _ := chatFramesIn(t, api.ViewChat, image.Pt(420, 800), busySession())
	r.settle(t)
	if !r.shows(i18n.T("chat.composer.send")) || !r.shows("PR #7") {
		t.Fatalf("the narrow composer lost Send or the first chips:\n%s", r.drawn())
	}
	if r.shows("10m") {
		t.Errorf("the last chip fits where it should have been cut:\n%s", r.drawn())
	}
	line := centreY(t, r, "PR #7")
	for _, label := range []string{i18n.T("chat.composer.send"), "42%"} {
		if !r.shows(label) {
			continue
		}
		if y := centreY(t, r, label); y < line-8 || y > line+8 {
			t.Errorf("%q wrapped to y=%v, the chips' line is y=%v", label, y, line)
		}
	}
}

func TestAnotherPaneOfALayoutHasNoFooterItems(t *testing.T) {
	in := busySession()
	in.sessionID = "s2"
	_, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	r.settle(t)
	if !r.shows(i18n.T("chat.composer.send")) {
		t.Fatal("the pane lost Send")
	}
	for _, label := range []string{"PR #7", "42%", "main"} {
		if r.shows(label) {
			t.Errorf("%q drawn in a pane the footer does not follow", label)
		}
	}
}
