package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// reviewRig is stageFrames on the Review tab of a checkout in, keeping the
// backend so a test can change the diff it answers.
func reviewRig(t *testing.T, in footerInput) (*reviewPanel, *frameRig, *sidebarBackend) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	u := &window{model: newModel(nil, r.invalidate), terms: &terminals{byID: map[string]*termView{}, invalidate: r.invalidate}}
	u.topbar = newTopbar(&u.root)
	u.sidebar = newSidebar(&u.root)
	f, client := newSidebarBackend(t)
	u.ctx, u.client = t.Context(), client
	comments := new(commentStore)
	code := newCodeTab(client, r.invalidate, comments, func(ui.Toast) {}, func(string, string, int) {})
	review, err := newReviewPanel(client, &u.root, comments, "monospace", 12, r.invalidate, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	u.dock = newDock(code, review, newArtifactsTab(new(artifactStore)), nil)
	u.dock.tab = dockReview
	u.footer = newFooter(u.ctx, client, &u.root, newFooterUsage(), newPlanQuota(quotaReader(client), r.invalidate), r.invalidate)
	u.footer.dock = u.dock
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			if err := u.stageRow(gtx, stageView{in: in}); err != nil {
				t.Fatal(err)
			}
			return D{Size: gtx.Constraints.Max}
		})
	}
	return review, r, f
}

func (f *sidebarBackend) setDiff(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.diff = text
}

var repoInput = footerInput{path: "/repo", checkout: "/repo", isGit: true}

// The review body replays while nothing changes; a diff read off the UI
// goroutine meanwhile still reaches the screen.
func TestReviewDrawsADiffThatArrivesWhileItsBodyReplays(t *testing.T) {
	p, r, f := reviewRig(t, repoInput)
	r.waitFor(t, i18n.T("diff.reviewPanel.noChanges"))
	r.frames(3)
	f.setDiff(twoHunks)
	p.feed.refresh()
	r.waitFor(t, "a.go")
}

func diffRig(t *testing.T, in footerInput) (*reviewPanel, *frameRig) {
	t.Helper()
	p, r, f := reviewRig(t, in)
	f.setDiff(twoHunks)
	r.waitFor(t, "a.go")
	r.frames(3)
	return p, r
}

func TestReviewCardTakesAClickAfterReplayedFrames(t *testing.T) {
	p, r := diffRig(t, repoInput)
	open := p.cards["a.go"].open
	r.clickLabel(t, "a.go")
	if p.cards["a.go"].open == open {
		t.Fatal("a click on a replayed card's title did not fold it")
	}
}

func TestReviewCardSeesAHoverAfterReplayedFrames(t *testing.T) {
	p, r := diffRig(t, repoInput)
	r.hover(r.at(t, "a.go"))
	if !p.cards["a.go"].toggle.Hovered() {
		t.Fatal("the pointer is on a replayed card's title, which does not see it hovered")
	}
}

// The commit button's tooltip waits out a delay: the frame it asked for
// comes while the body replays, and the tooltip shows.
func TestReviewTooltipShowsAfterItsDelayWhileTheBodyReplays(t *testing.T) {
	in := repoInput
	in.git = backend.DiffStats{Files: 1, Merging: "side", Conflicted: []string{"a.go"}}
	_, r := diffRig(t, in)
	blocked := i18n.T("git.merge.commitBlocked")
	r.hover(r.at(t, i18n.T("git.merge.commit")))
	for end := r.now.Add(time.Second); r.now.Before(end) && !r.shows(blocked); {
		r.now = r.now.Add(frameStep)
		r.frame()
	}
	if !r.shows(blocked) {
		t.Fatalf("the commit button's tooltip never showed:\n%s", r.drawn())
	}
}
