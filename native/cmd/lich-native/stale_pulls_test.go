package main

import (
	"encoding/json"
	"image"
	"net/http"
	"testing"

	"gioui.org/io/key"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const pullsDiff = "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1,1 +1,1 @@\n-old\n+new\n" +
	"diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1,1 +1,1 @@\n-old\n+new\n"

// pullsFrames is the Pulls screen on pull request 7 of /repo, framed the way
// the window frames it: its dialogs first, then the screen and its actions.
func pullsFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	r := newFrameRig(rigWindow, nil)
	conv := talkFixture()
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		switch method {
		case "project.ListPullRequests":
			return []backend.PRSummary{{Number: 7, Title: "Fix the thing", State: "OPEN"}, {Number: 8, Title: "Draft work", State: "OPEN", IsDraft: true}}, http.StatusOK
		case "project.PullRequestDetail":
			return backend.PRDetail{Number: 7, URL: "https://github.com/o/r/pull/7", Title: "Fix the thing", Body: "The body.", State: "OPEN", HeadRefName: "fix", BaseRefName: "main"}, http.StatusOK
		case "project.PullRequestConversation":
			return conv, http.StatusOK
		case "project.PullRequestDiff":
			return pullsDiff, http.StatusOK
		}
		return nil, http.StatusOK
	}}
	client := stub.client(t)
	u := &window{ctx: t.Context(), client: client, model: newModel(client, r.invalidate)}
	u.pulls = newPullsScreen(newPullsStore(t.Context(), client, r.invalidate), &u.root)
	u.pulls.sessionsIn = func(string, string) []string { return nil }
	u.pulls.files = filesInput{
		injector:  func(string) func(string) bool { return func(string) bool { return false } },
		submitter: func(string) func(string, func(error)) bool { return func(string, func(error)) bool { return false } },
	}
	files, err := newReviewPanel(client, &u.root, new(commentStore), "monospace", 12, r.invalidate, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	u.pulls.view.files = files
	u.pulls.view.files.pr = &prFiles{ticks: map[string]*widget.Bool{}}
	u.pulls.show(backend.Project{ID: "p", Path: "/repo"}, "main", 0)
	r.draw = func(gtx C, th *ui.Theme) {
		u.theme = th
		u.root.Layout(gtx, func(gtx C) D {
			u.layoutSubmitDialog(gtx, th)
			u.layoutMergeDialog(gtx, th)
			u.layoutDeleteCommentDialog(gtx, th)
			u.applyPulls(u.pulls.layout(gtx, th))
			return D{Size: gtx.Constraints.Max}
		})
	}
	return u, r
}

// pullsDetailShown opens pull request 7 from the index and waits for its
// overview.
func pullsDetailShown(t *testing.T) (*window, *frameRig) {
	t.Helper()
	u, r := pullsFrames(t)
	r.waitFor(t, "Fix the thing")
	r.clickLabel(t, "Fix the thing")
	r.waitFor(t, i18n.T("pulls.pullsOverview.openedBy", i18n.Args{"author": ""}))
	return u, r
}

func TestPullsSubmitCancelClosesInTheFrameAfterTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	u.pulls.view.submit.verdict = "approve"
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if u.pulls.view.submit.verdict != "" {
		t.Fatal("the click missed Cancel")
	}
}

func TestPullsMergeCancelClosesInTheFrameAfterTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	u.pulls.view.mergeEdit.show("squash", false, *u.pulls.store.detail("/repo", 7).val)
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if u.pulls.view.mergeEdit.open {
		t.Fatal("the click missed Cancel")
	}
}

func TestPullsFilterTabsFilterInTheFrameThatTookTheClick(t *testing.T) {
	u, r := pullsFrames(t)
	r.waitFor(t, "Fix the thing")
	r.clickLabel(t, pullsFilter(2).label())
	if u.pulls.filter.Selected != 2 {
		t.Fatal("the click missed Drafts")
	}
}

// openPullsTab opens the detail's tab named label and waits for want.
func openPullsTab(t *testing.T, r *frameRig, label, want string) {
	t.Helper()
	r.pressLabel(t, label)
	r.waitFor(t, want)
}

func TestPullsResolvedThreadsUnfoldInTheFrameThatTookTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	resolved := i18n.T("pulls.pullsConversation.resolved", i18n.Args{"count": 1})
	openPullsTab(t, r, tabConversation.label(), resolved)
	r.clickLabel(t, resolved)
	if !u.pulls.view.talk.showResolved.Value {
		t.Fatal("the click missed the resolved threads")
	}
}

func TestPullsPendingCommentDiscardsInTheFrameThatTookTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	pending := u.pulls.view.pendingOf("https://github.com/o/r/pull/7")
	pending.add("a.go", "3-5", "Rename this.")
	discard := i18n.T("pulls.pullsConversation.discard")
	openPullsTab(t, r, tabConversation.label(), discard)
	r.clickLabel(t, discard)
	if len(pending.snapshot()) != 0 {
		t.Fatal("the click missed Discard")
	}
}

func TestPullsReplyCancelClosesTheBoxInTheFrameThatTookTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	reply := i18n.T("pulls.reviewThread.reply")
	openPullsTab(t, r, tabConversation.label(), reply)
	r.clickLabel(t, reply)
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if u.pulls.view.talk.replying["T1"] {
		t.Fatal("the click missed Cancel")
	}
}

func TestPullsCommentEditTakesTheThreadsButtonsUntilCancel(t *testing.T) {
	u, r := pullsDetailShown(t)
	reply := i18n.T("pulls.reviewThread.reply")
	openPullsTab(t, r, tabConversation.label(), reply)
	conv := talkFixture()
	target := threadTarget(conv.Threads[0], conv.Threads[0].Comments[0])
	u.pulls.view.talk.startEdit(target)
	r.waitFor(t, i18n.T("pulls.reviewThread.save"))
	if got := u.pulls.view.talk.editorOf(target.key()).Text(); got != "Why not cap this?" {
		t.Errorf("edit box = %q, want the comment as it stands", got)
	}
	if r.shows(reply) {
		t.Error("Reply stayed under the comment being edited")
	}
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if u.pulls.view.talk.editing[target.key()] {
		t.Fatal("the click missed Cancel")
	}
	r.waitFor(t, reply)
}

func TestPullsDeleteDialogWarnsTheThreadGoesAndCancelCloses(t *testing.T) {
	u, r := pullsDetailShown(t)
	conv := talkFixture()
	target := threadTarget(conv.Threads[0], conv.Threads[0].Comments[0])
	u.pulls.view.talk.deleting = &target
	r.waitFor(t, i18n.T("pulls.reviewThread.deleteLast"))
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if u.pulls.view.talk.deleting != nil {
		t.Fatal("the click missed Cancel")
	}
}

func TestPullsDescriptionCancelInTheFrameThatTookTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	r.clickLabel(t, i18n.T("pulls.pullsOverview.editDescription"))
	r.clickLabel(t, i18n.T("common.action.cancel"))
	if u.pulls.view.edit.bodyURL != "" {
		t.Fatal("the click missed Cancel")
	}
}

func TestPullsViewedTickCountsInTheFrameThatTookTheClick(t *testing.T) {
	u, r := pullsDetailShown(t)
	viewed := i18n.T("diff.fileDiff.viewed")
	openPullsTab(t, r, tabFiles.label(), viewed)
	var label image.Point
	for _, n := range r.nodes() {
		if n.Desc.Label == viewed {
			label = n.Desc.Bounds.Min
			break
		}
	}
	r.click(t, r.smallestClickIn(t, image.Rect(label.X-40, label.Y-12, label.X+8, label.Y+30)))
	if len(u.pulls.view.files.pr.viewed) != 1 {
		t.Fatal("the click missed the tick")
	}
}

// The index opens on the screen, a row opens its pull request, and the
// crumb's back button returns to the index with the cursor on that row.
func TestPullsBackReturnsToTheIndexOnTheRowItLeft(t *testing.T) {
	u, r := pullsDetailShown(t)
	if u.pulls.selected != 7 {
		t.Fatalf("the click on the row opened #%d", u.pulls.selected)
	}
	r.clickLabel(t, i18n.T("tabs.projectTabs.pullRequests"))
	if u.pulls.selected != 0 || u.pulls.cursor != 7 {
		t.Fatalf("back left #%d open, cursor on #%d", u.pulls.selected, u.pulls.cursor)
	}
}

// j moves the index's cursor, Enter opens its row and Escape on the pull
// request goes back with the cursor where it was.
func TestPullsKeysWalkTheIndexAndEscapeGoesBack(t *testing.T) {
	u, r := pullsFrames(t)
	r.waitFor(t, "Fix the thing")
	press := func(name key.Name) {
		r.router.Queue(key.Event{Name: name, State: key.Press})
		r.settle(t)
	}
	if u.pulls.keyed {
		t.Fatal("the cursor's row is filled before any key moved it")
	}
	press("J")
	if u.pulls.cursor != 8 || !u.pulls.keyed {
		t.Fatalf("j left the cursor on #%d, keyed %v", u.pulls.cursor, u.pulls.keyed)
	}
	press(key.NameReturn)
	if u.pulls.selected != 8 {
		t.Fatalf("Enter opened #%d", u.pulls.selected)
	}
	press(key.NameEscape)
	if u.pulls.selected != 0 || u.pulls.cursor != 8 {
		t.Fatalf("Escape left #%d open, cursor on #%d", u.pulls.selected, u.pulls.cursor)
	}
}
