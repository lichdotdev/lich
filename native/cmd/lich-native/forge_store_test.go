package main

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// gitHubOnly are the reads a GitLab checkout must never reach.
var gitHubOnly = []string{
	"project.GitHubAccountOf", "project.PullRequestAvatar", "project.PullRequestConversation",
	"project.BranchRules", "project.PullRequestConflicts", "project.Issue",
}

func forgeAnswer(forge string) func(string, []json.RawMessage) (any, int) {
	return func(method string, _ []json.RawMessage) (any, int) {
		if method == "project.Forge" {
			return forge, http.StatusOK
		}
		return nil, http.StatusOK
	}
}

func TestForgeStoreReadsEachCheckoutOnce(t *testing.T) {
	stub := &rpcStub{answer: forgeAnswer(api.ForgeGitLab)}
	f := newForgeStore(stub.client(t), func() {})
	if _, known := f.of("/repo"); known {
		t.Fatal("known before the backend answered")
	}
	waitFor(t, "the forge", func() bool { forge, known := f.of("/repo"); return known && forge == api.ForgeGitLab })
	f.of("/repo")
	f.gitLab("/repo")
	if got := stub.calls("project.Forge"); got != 1 {
		t.Errorf("reads = %d, want 1: a change arrives as an event", got)
	}
}

func TestForgeStoreLeavesAFailedReadUnknown(t *testing.T) {
	stub := &rpcStub{answer: func(string, []json.RawMessage) (any, int) { return failing("backend restarting") }}
	f := newForgeStore(stub.client(t), func() {})
	f.of("/repo")
	waitFor(t, "the failed read", func() bool { return stub.calls("project.Forge") == 1 })
	if _, known := f.of("/repo"); known {
		t.Error("a failed read reads as a forge")
	}
	if f.gitLab("/repo") {
		t.Error("a failed read reads as GitLab")
	}
}

func TestPullsStoreAsksAGitLabCheckoutNothingGitHubAlone(t *testing.T) {
	stub := &rpcStub{answer: forgeAnswer(api.ForgeGitLab)}
	c := stub.client(t)
	forges := newForgeStore(c, func() {})
	waitFor(t, "the forge", func() bool { return forges.gitLab("/repo") })
	s := newPullsStore(context.Background(), c, forges, func() {})
	d := &backend.PRDetail{Number: 7, URL: "https://gitlab.com/o/r/-/merge_requests/7", BaseRefName: "main"}

	if a := s.account("/repo"); !a.loaded || accountGateOf(a.val) != gateNone {
		t.Errorf("account = %+v, want loaded with no gate", a)
	}
	if conv := s.conversation("/repo", 7); !conv.loaded || conv.val != nil {
		t.Errorf("conversation = %+v, want loaded and empty", conv)
	}
	if s.avatar("/repo", "https://gitlab.com/uploads/a.png") != nil || s.branchRules("/repo", "main") != nil {
		t.Error("a GitLab checkout drew a GitHub-only answer")
	}
	s.conflictFiles("/repo", d)
	s.list("/repo", pullsOpen)
	waitFor(t, "the list", func() bool { return s.list("/repo", pullsOpen).loaded })
	for _, m := range gitHubOnly {
		if got := stub.calls(m); got != 0 {
			t.Errorf("%s called %d times on a GitLab checkout", m, got)
		}
	}
}

func TestForgeKeyAndChangeRefFollowTheForge(t *testing.T) {
	if got := forgeKey(api.ForgeGitLab, "a.github", "a.gitlab"); got != "a.gitlab" {
		t.Errorf("GitLab key = %q", got)
	}
	for _, forge := range []string{api.ForgeGitHub, ""} {
		if got := forgeKey(forge, "a.github", "a.gitlab"); got != "a.github" {
			t.Errorf("forgeKey(%q) = %q, want the GitHub key", forge, got)
		}
		if got := changeRef(forge, 12); got != "#12" {
			t.Errorf("changeRef(%q) = %q, want #12", forge, got)
		}
	}
	if got := changeRef(api.ForgeGitLab, 12); got != "!12" {
		t.Errorf("GitLab ref = %q, want !12", got)
	}
}

func TestMergeRequestCopySaysMergeRequest(t *testing.T) {
	merged := detail("MERGED", "", "", "")
	if got := mergeBlockedReason(merged, api.ForgeGitLab); got != "Merge request is merged" {
		t.Errorf("merged = %q", got)
	}
	red := detail("OPEN", "MERGEABLE", "UNSTABLE", "")
	red.Checks.Failed = 1
	if got := mergeHint(red, nil, api.ForgeGitLab); got != "1 check is failing; GitLab will merge over it" {
		t.Errorf("failing = %q", got)
	}
	if got := pullsMerged.noneMergeRequests(); got != "No merged merge requests." {
		t.Errorf("empty merged list = %q", got)
	}
	d := backend.PRDetail{Number: 7, State: "OPEN", Mergeable: "CONFLICTING", HeadRefName: "feat/x", BaseRefName: "main"}
	if _, req, _ := pullHandoff(d, nil, api.ForgeGitLab); req.Forge != api.ForgeGitLab || req.Topic != api.RequestConflicts {
		t.Errorf("hand-off = %+v, want a GitLab conflicts request", req)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := mergeBlockedReason(merged, api.ForgeGitLab); got != "O merge request já foi mergeado" {
		t.Errorf("pt-BR merged = %q", got)
	}
}

func TestMergeRequestShowsNoConversationTab(t *testing.T) {
	d := backend.PRDetail{Checks: backend.ChecksRollup{Total: 1}}
	if got := tabsOf(d, api.ForgeGitLab); slices.Contains(got, tabConversation) || len(got) != 4 {
		t.Errorf("GitLab tabs = %v, want Overview, Commits, Files, Checks", got)
	}
	if got := tabsOf(d, api.ForgeGitHub); !slices.Contains(got, tabConversation) {
		t.Errorf("GitHub tabs = %v, want Conversation", got)
	}
}

func TestMergeRequestActionKeepsOnlyWhatGitLabTakes(t *testing.T) {
	body := "new"
	fix := &api.RequestOptions{Topic: api.RequestChecks}
	all := pullsAction{session: &backend.PRDetail{}, handoff: fix, merge: "squash", admin: true, approve: true, refresh: true,
		external: "u", create: true, comment: "c", reply: &threadReply{}, resolve: &threadResolve{}, threads: []backend.PRReviewThread{{}},
		edit: &commentEdit{}, title: "t", body: &body}
	got := mergeRequestAction(all)
	want := pullsAction{session: all.session, handoff: fix, merge: "squash", refresh: true, external: "u", create: true}
	if got.admin || got.approve || got.comment != "" || got.reply != nil || got.resolve != nil || got.threads != nil ||
		got.edit != nil || got.title != "" || got.body != nil {
		t.Errorf("kept a GitHub-only action: %+v", got)
	}
	if got.session != want.session || got.handoff != want.handoff || got.merge != want.merge || got.refresh != want.refresh ||
		got.external != want.external || got.create != want.create {
		t.Errorf("dropped a supported action: %+v", got)
	}
}

func TestMergeRequestMergeMessageIsGitLabs(t *testing.T) {
	d := backend.PRDetail{Number: 7, Title: "feat: x", Body: "why", HeadRefName: "feat/x", BaseRefName: "main"}
	if got := mergeRequestEditFor("squash", d); got.subject != "feat: x" || got.body != "why" {
		t.Errorf("squash = %+v", got)
	}
	if got := mergeRequestEditFor("merge", d); got.subject != "Merge branch 'feat/x' into 'main'" || got.body != "" {
		t.Errorf("merge = %+v", got)
	}
}

func TestMergeRequestFileURLPointsAtGitLabsChanges(t *testing.T) {
	got := mergeRequestFileURL("https://gitlab.com/o/r/-/merge_requests/7/", "a/b.png")
	if want := "https://gitlab.com/o/r/-/merge_requests/7/diffs#7f9c894b6fccf26aac3e8796263d363abe8cb84e"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
}

func TestWorktreeDialogLooksUpNoGitLabIssue(t *testing.T) {
	stub := &rpcStub{answer: forgeAnswer(api.ForgeGitLab)}
	c := stub.client(t)
	forges := newForgeStore(c, func() {})
	waitFor(t, "the forge", func() bool { return forges.gitLab("/repo") })
	d := newWorktreeDialog(c, forges, new(ui.Root), func() {})
	d.project = backend.Project{Path: "/repo"}
	d.lookUpIssue(context.Background(), "#12")
	if d.issueRef != 0 {
		t.Fatalf("issueRef = %d, want #12 kept as a plain name", d.issueRef)
	}
	if got := stub.calls("project.Issue"); got != 0 {
		t.Errorf("Issue called %d times on a GitLab project", got)
	}
}

func TestPullsScreenOnGitLabWaitsForTheForgeAndNeverAsksGitHub(t *testing.T) {
	hold := make(chan struct{})
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(hold) }) })
	stub := &rpcStub{answer: func(method string, _ []json.RawMessage) (any, int) {
		switch method {
		case "project.Forge":
			<-hold
			return api.ForgeGitLab, http.StatusOK
		case "project.ListPullRequests":
			return []backend.PRSummary{{Number: 7, Title: "Fix the thing", State: "OPEN"}}, http.StatusOK
		case "project.PullRequestDetail":
			return backend.PRDetail{Number: 7, URL: "https://gitlab.com/o/r/-/merge_requests/7", Title: "Fix the thing", State: "OPEN", HeadRefName: "fix", BaseRefName: "main"}, http.StatusOK
		}
		return nil, http.StatusOK
	}}
	r := newFrameRig(rigWindow, nil)
	client := stub.client(t)
	u := &window{ctx: t.Context(), client: client, model: newModel(client, r.invalidate)}
	u.pulls = newPullsScreen(newPullsStore(t.Context(), client, u.model.forges, r.invalidate), &u.root)
	u.pulls.sessionsIn = func(string, string) []string { return nil }
	u.pulls.show(backend.Project{ID: "p", Path: "/repo"}, "main", 0)
	r.draw = func(gtx C, th *ui.Theme) {
		u.root.Layout(gtx, func(gtx C) D {
			u.applyPulls(u.pulls.layout(gtx, th))
			return D{Size: gtx.Constraints.Max}
		})
	}
	r.frames(3)
	if got := stub.calls("project.GitHubAccountOf"); got != 0 {
		t.Fatalf("asked gh for an account before the forge was known: %d", got)
	}
	release.Do(func() { close(hold) })
	r.waitFor(t, "!7")
	r.clickLabel(t, "Fix the thing")
	r.waitFor(t, "Merge requests")
	if r.shows(i18n.T("pulls.pullRequestView.tab.conversation")) {
		t.Error("a merge request shows Conversation")
	}
	for _, m := range gitHubOnly {
		if got := stub.calls(m); got != 0 {
			t.Errorf("%s called %d times on a GitLab checkout", m, got)
		}
	}
}

// A checkout read as GitHub turns GitLab when the backend announces it, as
// after `glab auth login` on a self-hosted host, with no read of its own.
func TestAForgeEventTurnsACheckoutGitLab(t *testing.T) {
	stub := &rpcStub{answer: forgeAnswer(api.ForgeGitHub)}
	m := newTestModel(stub.client(t))
	waitFor(t, "the first read", func() bool { return m.forgeOf(checkout) == api.ForgeGitHub })
	send(m, api.ForgeEvent.Name, api.ForgeChange{Path: checkout, Forge: api.ForgeGitLab})
	if got := m.forgeOf(checkout); got != api.ForgeGitLab {
		t.Errorf("forge after the event = %q, want gitlab", got)
	}
	if got := stub.calls("project.Forge"); got != 1 {
		t.Errorf("reads = %d, want the first alone", got)
	}
}
