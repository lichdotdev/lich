package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui"
)

func linkPanelFrames(t *testing.T) (*linkPRPanel, *frameRig, *chatBackend) {
	t.Helper()
	f, client := newChatBackend(t)
	p := new(linkPRPanel)
	p.open(sessionView{ID: "s1", path: "/wt", forge: api.ForgeGitHub})
	p.panel.OpenAt(image.Pt(20, 20))
	root := new(ui.Root)
	var r *frameRig
	r = newFrameRig(image.Pt(600, 400), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			p.layout(t.Context(), gtx, th, root, client, r.invalidate)
			return D{Size: gtx.Constraints.Max}
		})
	})
	r.frame()
	r.frame()
	return p, r, f
}

// Enter on an empty field links nothing; on a reference it links it to the
// card's session, read in its checkout, and the panel closes once kept.
func TestTheLinkPanelLinksWhatWasTyped(t *testing.T) {
	p, r, f := linkPanelFrames(t)
	if !r.shows("#1214 or https://github.com/…/pull/1214") || !r.shows("Cancel") || !r.shows("Link") {
		t.Fatal("the panel did not open with its field and buttons")
	}
	r.press(key.NameReturn)
	p.mu.Lock()
	busy := p.busy
	p.mu.Unlock()
	if busy || f.callsOf(api.ProjectLinkPullRequest.Name) != 0 {
		t.Fatal("Enter on an empty field asked for a link")
	}
	p.field.SetText("  #1214 ")
	r.frame()
	r.press(key.NameReturn)
	body := f.waitCall(t, api.ProjectLinkPullRequest.Name)
	if want := `{"session":"s1","path":"/wt","ref":"#1214"}`; !strings.Contains(body, want) {
		t.Errorf("link asked %s, want %s", body, want)
	}
	deadline := time.Now().Add(5 * time.Second)
	for p.panel.Opened() && time.Now().Before(deadline) {
		r.frame()
		time.Sleep(10 * time.Millisecond)
	}
	if p.panel.Opened() {
		t.Error("the panel stayed open after the link was kept")
	}
}

func TestEscapeClosesTheLinkPanel(t *testing.T) {
	p, r, _ := linkPanelFrames(t)
	r.press(key.NameEscape)
	r.frame()
	if p.panel.Opened() {
		t.Error("Escape left the panel open")
	}
}

// A link answered after the panel opened for another card is not this one's.
func TestALateLinkAnswerIsDropped(t *testing.T) {
	p := new(linkPRPanel)
	p.open(sessionView{ID: "s1"})
	p.mu.Lock()
	gen := p.gen
	p.mu.Unlock()
	p.open(sessionView{ID: "s2"})
	if gen == p.gen || p.busy || p.done || p.err != "" {
		t.Errorf("reopen = gen %d (was %d), busy %v, done %v, err %q; want a fresh open", p.gen, gen, p.busy, p.done, p.err)
	}
}
