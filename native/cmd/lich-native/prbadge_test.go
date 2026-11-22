package main

import (
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

type prCase struct {
	name   string
	pr     backend.PullRequest
	logo   *icons.Icon
	glyph  *icons.Icon
	tone   func(*ui.Theme) any
	number string
}

func prCases() []prCase {
	pass := func(th *ui.Theme) any { return th.TonePass }
	return []prCase{
		{"open", backend.PullRequest{Number: 990, State: "OPEN", Forge: api.ForgeGitHub}, icGitHub, icPROpen, pass, "#990"},
		{"no state", backend.PullRequest{Number: 990, Forge: api.ForgeGitHub}, icGitHub, icPROpen, pass, "#990"},
		{"draft", backend.PullRequest{Number: 990, State: "OPEN", IsDraft: true, Forge: api.ForgeGitHub}, icGitHub, icPRDraft, func(th *ui.Theme) any { return th.MutedForeground }, "#990"},
		{"merged", backend.PullRequest{Number: 990, State: "MERGED", Forge: api.ForgeGitHub}, icGitHub, icPRMerged, func(th *ui.Theme) any { return th.ToneMerged }, "#990"},
		{"closed draft", backend.PullRequest{Number: 990, State: "CLOSED", IsDraft: true, Forge: api.ForgeGitHub}, icGitHub, icPRClosed, func(th *ui.Theme) any { return th.Destructive }, "#990"},
		{"gitlab", backend.PullRequest{Number: 12, State: "OPEN", Forge: api.ForgeGitLab}, icGitLab, icPROpen, pass, "!12"},
		{"unknown forge", backend.PullRequest{Number: 990, State: "OPEN"}, icPR, icPROpen, pass, "#990"},
	}
}

func TestPRLookFollowsStateAndForge(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	for _, c := range prCases() {
		l := prLookOf(th, &c.pr)
		if l.logo != c.logo || l.glyph != c.glyph || any(l.tone) != c.tone(th) || l.number != c.number {
			t.Errorf("%s: logo %p glyph %p tone %v number %q; want logo %p glyph %p tone %v number %q",
				c.name, l.logo, l.glyph, l.tone, l.number, c.logo, c.glyph, c.tone(th), c.number)
		}
	}
}

func TestOnlyMergedAndClosedPRsAreNotOpen(t *testing.T) {
	for state, open := range map[string]bool{"": true, "OPEN": true, "MERGED": false, "CLOSED": false} {
		if got := isOpenPR(&backend.PullRequest{State: state}); got != open {
			t.Errorf("isOpenPR(%q) = %v, want %v", state, got, open)
		}
	}
	if isOpenPR(nil) {
		t.Error("isOpenPR(nil) = true")
	}
}

// The session's row carries no pull request: it lives in the tooltip and on
// the chat composer's line.
func TestTheRowDrawsNoPullRequest(t *testing.T) {
	sv := sessionView{ID: "s1", Label: "Docs", shown: "/repo", git: backend.DiffStats{Branch: "feat"},
		pr: &backend.PullRequest{Number: 990, State: "MERGED", Forge: api.ForgeGitHub}}
	s := newSidebar(&ui.Root{})
	more := func(gtx C) D { return D{Size: image.Pt(16, 16)} }
	r := newFrameRig(image.Pt(300, 40), func(gtx C, th *ui.Theme) { s.cardBody(gtx, th, &sv, more, false) })
	r.frame()
	if r.shows("#990") {
		t.Error("the row drew its pull request")
	}
}

// The tooltip draws every state with its number, and no two states alike.
func TestTheTooltipDrawsEachPRState(t *testing.T) {
	paints := map[string]string{}
	for _, c := range prCases() {
		if c.name == "no state" {
			continue
		}
		sv := sessionView{ID: "s1", Label: "Docs", shown: "/repo", git: backend.DiffStats{Branch: "feat"}, pr: &c.pr}
		r := newFrameRig(image.Pt(400, 120), func(gtx C, th *ui.Theme) { sessionTooltip(th, &sv)(gtx) })
		drawn, _ := r.frame()
		if !r.shows(c.number) {
			t.Errorf("%s: no %q drawn:\n%s", c.name, c.number, drawn)
		}
		paint, _, _ := strings.Cut(drawn, "\n")
		if other, ok := paints[paint]; ok {
			t.Errorf("%s draws the same as %s", c.name, other)
		}
		paints[paint] = c.name
	}
}

const longBranch = "feat/sidebar-pull-request-badge-closed-state-and-then-some-more-words-past-the-edge"

// A branch longer than the tooltip is wide wraps on a line of its own, and
// the pull request and the changes stand whole on the line under it.
func TestTheTooltipKeepsTheBranchWholeOnItsLine(t *testing.T) {
	width := 240
	// drop is how far under the branch's first line the pull request stands
	drop := func(branch string) (image.Rectangle, image.Rectangle) {
		sv := sessionView{ID: "s1", Label: "Docs", shown: "/repo", git: backend.DiffStats{Branch: branch, Files: 2, Added: 10, Deleted: 3},
			pr: &backend.PullRequest{Number: 1199, State: "CLOSED", Forge: api.ForgeGitHub}}
		r := newFrameRig(image.Pt(width, 300), func(gtx C, th *ui.Theme) { sessionTooltip(th, &sv)(gtx) })
		r.frame()
		number, added := labelBox(t, r, "#1199"), labelBox(t, r, "+10")
		if added.Min.Y != number.Min.Y || added.Min.X <= number.Min.X {
			t.Errorf("%s: +10 at %v, want the changes after #1199 (%v) on its line", branch, added, number)
		}
		if number.Min.X > width/2 {
			t.Errorf("%s: #1199 at %v, want it at the start of its line", branch, number)
		}
		return labelBox(t, r, branch), number
	}
	shortBranch, shortNumber := drop("feat")
	longBranchBox, longNumber := drop(longBranch)
	one := shortNumber.Min.Y - shortBranch.Min.Y
	if got := longNumber.Min.Y - longBranchBox.Min.Y; got < 2*one {
		t.Errorf("under a long branch #1199 stands %dpx down, under a short one %dpx: want the long one wrapped onto more lines", got, one)
	}
}

// composerWithPR is a chat whose session has pr, on a stage of size, its
// footer following follows.
func composerWithPR(t *testing.T, size image.Point, pr backend.PullRequest, follows string) (*window, *frameRig, *chatBackend) {
	in := busySession()
	in.sessionID = follows
	in.git.Branch = longBranch
	u, r, f := chatFramesIn(t, api.ViewChat, size, in)
	m := u.model
	m.git[m.shownPath(m.projects[0], m.projects[0].Sessions[0])] = in.git
	m.prs[m.shownPath(m.projects[0], m.projects[0].Sessions[0])] = prLookup{pr: &pr}
	r.settle(t)
	return u, r, f
}

// The composer's line draws each state after the changes.
func TestTheComposerDrawsEachPRStateAfterTheChanges(t *testing.T) {
	for _, c := range prCases() {
		pr := c.pr
		pr.Title = "Add the badge"
		_, r, _ := composerWithPR(t, rigWindow, pr, "s1")
		if !r.shows(c.number) {
			t.Errorf("%s: no %q on the composer's line", c.name, c.number)
			continue
		}
		changes, number := labelBox(t, r, "+0"), labelBox(t, r, c.number)
		if number.Min.Y != changes.Min.Y || number.Min.X <= changes.Min.X {
			t.Errorf("%s: %q at %v, the changes at %v: want it after them on their line", c.name, c.number, number, changes)
		}
	}
}

// In a narrow pane the branch truncates, and what follows it stays inside.
func TestTheComposerNeverCutsThePullRequest(t *testing.T) {
	pr := backend.PullRequest{Number: 1187, State: "MERGED", Forge: api.ForgeGitHub}
	_, r, _ := composerWithPR(t, image.Pt(300, 800), pr, "s1")
	branch, changes, number := labelBox(t, r, longBranch), labelBox(t, r, "+0"), labelBox(t, r, "#1187")
	if !(branch.Min.X < changes.Min.X && changes.Min.X < number.Min.X) {
		t.Errorf("branch %v, changes %v, #1187 %v: want them in that order", branch, changes, number)
	}
	// "#1187" in TextXS is under 40px wide
	if number.Min.X > r.size.X-40 {
		t.Errorf("#1187 starts at x=%d of a %dpx pane: the line ran off its edge", number.Min.X, r.size.X)
	}
}

// A click on the composer's pull request opens it; another pane's takes none.
func TestTheComposerPullRequestOpensOnlyInTheFollowedPane(t *testing.T) {
	pr := backend.PullRequest{Number: 1187, State: "MERGED", Forge: api.ForgeGitHub, URL: "https://github.com/acme/app/pull/1187"}
	_, r, f := composerWithPR(t, rigWindow, pr, "s1")
	r.click(t, r.at(t, "#1187"))
	if body := f.waitCall(t, api.SystemOpenExternal.Name); !strings.Contains(body, pr.URL) {
		t.Errorf("opened %s, want %s", body, pr.URL)
	}

	_, r, f = composerWithPR(t, rigWindow, pr, "another")
	r.click(t, r.at(t, "#1187"))
	time.Sleep(100 * time.Millisecond)
	if n := f.callsOf(api.SystemOpenExternal.Name); n != 0 {
		t.Errorf("another pane's pull request opened %d times", n)
	}
}

// Hovering the composer's pull request brightens its logo and leaves the
// state's glyph and number in their tone.
func TestHoverBrightensOnlyTheLogo(t *testing.T) {
	pr := backend.PullRequest{Number: 1187, State: "MERGED", Forge: api.ForgeGitHub}
	_, r, _ := composerWithPR(t, rigWindow, pr, "s1")
	rest := renderRig(t, r)
	number := labelBox(t, r, "#1187")
	r.hover(f32.Pt(float32(number.Min.X+2), float32(number.Min.Y+6)))
	r.frame()
	hovered := renderRig(t, r)
	// the logo and the glyph stand 12px wide with a 4px gap before each
	logo := image.Rect(number.Min.X-32, number.Min.Y-4, number.Min.X-20, number.Min.Y+16)
	tinted := image.Rect(number.Min.X-16, number.Min.Y-4, number.Min.X+40, number.Min.Y+16)
	if inkShift(rest, hovered, logo) == 0 {
		t.Error("hover left the logo as it was")
	}
	if d := inkShift(rest, hovered, tinted); d != 0 {
		t.Errorf("hover moved the glyph or the number by %d", d)
	}
}
