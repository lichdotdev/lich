package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

func summary(n int, title string, draft bool, failed int, updated string) backend.PRSummary {
	return backend.PRSummary{
		Number: n, Title: title, Author: "octo", HeadRefName: "feat/" + title, IsDraft: draft, UpdatedAt: updated,
		Checks: backend.ChecksRollup{Failed: failed, Total: max(failed, 1)},
	}
}

func numbers(rows []backend.PRSummary) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.Number
	}
	return out
}

func TestFilterPullsKeepsWhatTheFilterAndTextAgreeOnNewestFirst(t *testing.T) {
	list := []backend.PRSummary{
		summary(1, "old", false, 0, "2026-01-01T00:00:00Z"),
		summary(2, "draft", true, 0, "2026-03-01T00:00:00Z"),
		summary(3, "red", false, 2, "2026-02-01T00:00:00Z"),
	}
	cases := []struct {
		filter pullsFilter
		text   string
		want   []int
	}{
		{filterAll, "", []int{2, 3, 1}},
		{filterReady, "", []int{3, 1}},
		{filterDrafts, "", []int{2}},
		{filterFailing, "", []int{3}},
		{filterAll, "#3", []int{3}},
		{filterAll, "1", []int{1}},
		{filterAll, "feat/dr", []int{2}},
		{filterAll, "OCTO", []int{2, 3, 1}},
		{filterReady, "draft", nil},
	}
	for _, c := range cases {
		if got := numbers(filterPulls(list, c.filter, c.text)); !slices.Equal(got, c.want) {
			t.Errorf("filterPulls(%v, %q) = %v, want %v", c.filter, c.text, got, c.want)
		}
	}
	if got := filterCounts(list, ""); got != [4]int{3, 2, 1, 1} {
		t.Errorf("filterCounts = %v, want [3 2 1 1]", got)
	}
	if got := filterCounts(list, "red"); got != [4]int{1, 1, 0, 1} {
		t.Errorf("filterCounts(red) = %v, want [1 1 0 1]", got)
	}
}

func detail(state, mergeable, status, review string) backend.PRDetail {
	return backend.PRDetail{State: state, Mergeable: mergeable, MergeStateStatus: status, ReviewDecision: review, BaseRefName: "main"}
}

func TestMergeBlockedReasonFollowsTheWebOrder(t *testing.T) {
	draft := detail("OPEN", "MERGEABLE", "DRAFT", "")
	draft.IsDraft = true
	cases := map[string]struct {
		d    backend.PRDetail
		want string
	}{
		"merged":            {detail("MERGED", "", "", ""), "Pull request is merged"},
		"closed":            {detail("CLOSED", "", "", ""), "Pull request is closed"},
		"draft":             {draft, "Pull request is a draft"},
		"conflicting":       {detail("OPEN", "CONFLICTING", "BLOCKED", "APPROVED"), "Conflicts with main"},
		"dirty":             {detail("OPEN", "UNKNOWN", "DIRTY", ""), "Conflicts with main"},
		"behind":            {detail("OPEN", "MERGEABLE", "BEHIND", ""), "Base branch has moved; update this branch first"},
		"changes requested": {detail("OPEN", "MERGEABLE", "BLOCKED", "CHANGES_REQUESTED"), "A reviewer asked for changes"},
		"review required":   {detail("OPEN", "MERGEABLE", "BLOCKED", "REVIEW_REQUIRED"), "GitHub requires a review that has not been left"},
		"blocked unknown":   {detail("OPEN", "MERGEABLE", "BLOCKED", "APPROVED"), ""},
		"clean":             {detail("OPEN", "MERGEABLE", "CLEAN", ""), ""},
	}
	for name, c := range cases {
		if got := mergeBlockedReason(c.d, api.ForgeGitHub); got != c.want {
			t.Errorf("%s: mergeBlockedReason = %q, want %q", name, got, c.want)
		}
	}
}

func TestAdminOverrideNeedsThePrivilegeAndNoConflict(t *testing.T) {
	admin := &backend.BranchRules{ViewerCanBypass: true}
	if !canAdminOverride(detail("OPEN", "MERGEABLE", "BLOCKED", ""), admin) {
		t.Error("an administrator should merge past BLOCKED")
	}
	if !canAdminOverride(detail("OPEN", "MERGEABLE", "BEHIND", ""), admin) {
		t.Error("an administrator should merge past BEHIND")
	}
	if canAdminOverride(detail("OPEN", "CONFLICTING", "BLOCKED", ""), admin) {
		t.Error("no bypass merges past a conflict")
	}
	if canAdminOverride(detail("OPEN", "MERGEABLE", "CLEAN", ""), admin) {
		t.Error("a clean PR has nothing to bypass")
	}
	if canAdminOverride(detail("OPEN", "MERGEABLE", "BLOCKED", ""), &backend.BranchRules{}) {
		t.Error("no privilege, no bypass")
	}
	if canAdminOverride(detail("OPEN", "MERGEABLE", "BLOCKED", ""), nil) {
		t.Error("unread rules fail closed")
	}
}

func TestAllowedMergeMethods(t *testing.T) {
	if got := allowedMergeMethods(nil); !slices.Equal(got, everyMergeMethod) {
		t.Errorf("no rules = %v, want every method", got)
	}
	r := &backend.BranchRules{AllowedMergeMethods: []string{"rebase", "squash"}}
	if got := allowedMergeMethods(r); !slices.Equal(got, []string{"squash", "rebase"}) {
		t.Errorf("named = %v, want [squash rebase] in the menu's order", got)
	}
}

func TestMergeHintFallsFromBlockToRuleToChecks(t *testing.T) {
	rules := &backend.BranchRules{CommitPatterns: []backend.CommitPattern{{Target: "message", Operator: "starts_with", Pattern: "feat"}}}
	blocked := detail("OPEN", "MERGEABLE", "BLOCKED", "APPROVED")
	if got := mergeHint(blocked, rules, api.ForgeGitHub); got != `A ruleset governs main: every commit message must start with "feat".` {
		t.Errorf("rule note = %q", got)
	}
	red := detail("OPEN", "MERGEABLE", "UNSTABLE", "")
	red.Checks.Failed = 2
	if got := mergeHint(red, nil, api.ForgeGitHub); !strings.HasPrefix(got, "2 checks are failing") {
		t.Errorf("failing hint = %q", got)
	}
	if got := mergeHint(detail("OPEN", "MERGEABLE", "CLEAN", ""), nil, api.ForgeGitHub); got != "" {
		t.Errorf("clean hint = %q, want none", got)
	}
}

func TestPullHandoffConflictsFirstThenChecks(t *testing.T) {
	d := detail("OPEN", "CONFLICTING", "DIRTY", "")
	d.Number, d.HeadRefName = 7, "feat/x"
	d.Checks.Failed = 1
	label, req, ok := pullHandoff(d, nil, api.ForgeGitHub)
	if !ok || label != "Resolve conflicts" || req.Topic != api.RequestConflicts || req.PR != 7 || req.Branch != "feat/x" || req.Base != "main" {
		t.Fatalf("conflict handoff = %q %+v %v", label, req, ok)
	}
	if _, req, _ = pullHandoff(d, []string{"go.mod", "a/b.go"}, api.ForgeGitHub); len(req.Items) != 2 || req.Items[1].Path != "a/b.go" {
		t.Errorf("conflict handoff names no files: %+v", req)
	}
	d.Mergeable, d.MergeStateStatus = "MERGEABLE", "UNSTABLE"
	d.CheckRuns = []backend.CheckItem{{Name: "job", State: "failed", URL: "https://ci"}, {Name: "ok", State: "passed"}}
	label, req, _ = pullHandoff(d, nil, api.ForgeGitHub)
	if label != "Fix CI errors" || req.Topic != api.RequestChecks || req.Failed != 1 ||
		len(req.Items) != 1 || req.Items[0].Check != "job" || req.Items[0].URL != "https://ci" {
		t.Errorf("checks handoff = %q %+v", label, req)
	}
	if _, _, ok := pullHandoff(detail("OPEN", "MERGEABLE", "CLEAN", ""), nil, api.ForgeGitHub); ok {
		t.Error("a healthy PR has nothing to hand off")
	}
	if _, _, ok := pullHandoff(detail("MERGED", "CONFLICTING", "DIRTY", ""), nil, api.ForgeGitHub); ok {
		t.Error("a merged PR has nothing to hand off")
	}
}

func TestCheckDuration(t *testing.T) {
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	cases := map[[2]string]string{
		{"2026-01-01T00:59:30Z", "2026-01-01T00:59:50Z"}: "20s",
		{"2026-01-01T00:50:00Z", "2026-01-01T00:52:00Z"}: "2m",
		{"2026-01-01T00:50:00Z", "2026-01-01T00:52:05Z"}: "2m 5s",
		{"2026-01-01T00:00:00Z", ""}:                     "1h",
		{"2025-12-31T22:30:00Z", "2026-01-01T00:00:00Z"}: "1h 30m",
		{"", ""}: "",
		{"2026-01-01T00:59:50Z", "2026-01-01T00:59:30Z"}: "",
	}
	for in, want := range cases {
		if got := checkDuration(in[0], in[1], now); got != want {
			t.Errorf("checkDuration(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestMergeEditStartsFromGitHubsOwnMessage(t *testing.T) {
	d := backend.PRDetail{Number: 7, Title: "feat: x", Body: "why", HeadRefName: "feat/x"}
	if got := mergeEditFor("squash", d); got.subject != "feat: x (#7)" || got.body != "why" || got.title != "Squash and merge" {
		t.Errorf("squash edit = %+v", got)
	}
	if got := mergeEditFor("merge", d); got.subject != "Merge pull request #7 from feat/x" || got.body != "" {
		t.Errorf("merge edit = %+v", got)
	}
	var v pullView
	entries := v.editEntries(everyMergeMethod, false, d)
	if len(entries) != 3 || entries[0].Kind != ui.MenuSeparator || !strings.HasPrefix(entries[1].Label, "Squash and merge, edit") {
		t.Fatalf("entries = %+v, want a separator then squash and merge commit, no rebase", entries)
	}
	if got := v.editEntries([]string{"rebase"}, false, d); got != nil {
		t.Errorf("rebase alone offers %+v, want nothing", got)
	}
	entries[2].OnClick()
	if !v.mergeEdit.open || v.mergeEdit.edit.method != "merge" || v.mergeEdit.subject.Text() != "Merge pull request #7 from feat/x" {
		t.Errorf("choosing the edit left the dialog %+v", v.mergeEdit.edit)
	}
}
