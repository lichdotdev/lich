package main

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

func summary(n int, title string, draft bool, failed int, updated string) lichclient.PRSummary {
	return lichclient.PRSummary{
		Number: n, Title: title, Author: "octo", HeadRefName: "feat/" + title, IsDraft: draft, UpdatedAt: updated,
		Checks: lichclient.ChecksRollup{Failed: failed, Total: max(failed, 1)},
	}
}

func numbers(rows []lichclient.PRSummary) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.Number
	}
	return out
}

func TestFilterPullsKeepsWhatTheFilterAndTextAgreeOnNewestFirst(t *testing.T) {
	list := []lichclient.PRSummary{
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

func detail(state, mergeable, status, review string) lichclient.PRDetail {
	return lichclient.PRDetail{State: state, Mergeable: mergeable, MergeStateStatus: status, ReviewDecision: review, BaseRefName: "main"}
}

func TestMergeBlockedReasonFollowsTheWebOrder(t *testing.T) {
	draft := detail("OPEN", "MERGEABLE", "DRAFT", "")
	draft.IsDraft = true
	cases := map[string]struct {
		d    lichclient.PRDetail
		want string
	}{
		"merged":            {detail("MERGED", "", "", ""), pullsMergedText},
		"closed":            {detail("CLOSED", "", "", ""), pullsClosedText},
		"draft":             {draft, pullsDraftText},
		"conflicting":       {detail("OPEN", "CONFLICTING", "BLOCKED", "APPROVED"), "Conflicts with main"},
		"dirty":             {detail("OPEN", "UNKNOWN", "DIRTY", ""), "Conflicts with main"},
		"behind":            {detail("OPEN", "MERGEABLE", "BEHIND", ""), pullsBehindText},
		"changes requested": {detail("OPEN", "MERGEABLE", "BLOCKED", "CHANGES_REQUESTED"), pullsChangesText},
		"review required":   {detail("OPEN", "MERGEABLE", "BLOCKED", "REVIEW_REQUIRED"), pullsReviewReqText},
		"blocked unknown":   {detail("OPEN", "MERGEABLE", "BLOCKED", "APPROVED"), ""},
		"clean":             {detail("OPEN", "MERGEABLE", "CLEAN", ""), ""},
	}
	for name, c := range cases {
		if got := mergeBlockedReason(c.d); got != c.want {
			t.Errorf("%s: mergeBlockedReason = %q, want %q", name, got, c.want)
		}
	}
}

func TestAdminOverrideNeedsThePrivilegeAndNoConflict(t *testing.T) {
	admin := &lichclient.BranchRules{ViewerCanBypass: true}
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
	if canAdminOverride(detail("OPEN", "MERGEABLE", "BLOCKED", ""), &lichclient.BranchRules{}) {
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
	r := &lichclient.BranchRules{AllowedMergeMethods: []string{"rebase", "squash"}}
	if got := allowedMergeMethods(r); !slices.Equal(got, []string{"squash", "rebase"}) {
		t.Errorf("named = %v, want [squash rebase] in the menu's order", got)
	}
}

func TestMergeHintFallsFromBlockToRuleToChecks(t *testing.T) {
	rules := &lichclient.BranchRules{CommitPatterns: []lichclient.CommitPattern{{Target: "message", Operator: "starts_with", Pattern: "feat"}}}
	blocked := detail("OPEN", "MERGEABLE", "BLOCKED", "APPROVED")
	if got := mergeHint(blocked, rules); got != `A ruleset governs main: every commit message must start with "feat".` {
		t.Errorf("rule note = %q", got)
	}
	red := detail("OPEN", "MERGEABLE", "UNSTABLE", "")
	red.Checks.Failed = 2
	if got := mergeHint(red, nil); !strings.HasPrefix(got, "2 checks are failing") {
		t.Errorf("failing hint = %q", got)
	}
	if got := mergeHint(detail("OPEN", "MERGEABLE", "CLEAN", ""), nil); got != "" {
		t.Errorf("clean hint = %q, want none", got)
	}
}

func TestPullHandoffConflictsFirstThenChecks(t *testing.T) {
	d := detail("OPEN", "CONFLICTING", "DIRTY", "")
	d.Number, d.HeadRefName = 7, "feat/x"
	d.Checks.Failed = 1
	label, prompt, ok := pullHandoff(d)
	if !ok || label != "Resolve conflicts" || !strings.Contains(prompt, "conflicts with main") {
		t.Fatalf("conflict handoff = %q %q %v", label, prompt, ok)
	}
	d.Mergeable, d.MergeStateStatus = "MERGEABLE", "UNSTABLE"
	runs := make([]lichclient.CheckItem, handoffNamedChecks+2)
	for i := range runs {
		runs[i] = lichclient.CheckItem{Name: "job", State: "failed", URL: "https://ci"}
	}
	d.CheckRuns = runs
	label, prompt, _ = pullHandoff(d)
	if label != "Fix CI errors" || strings.Count(prompt, "- job — https://ci") != handoffNamedChecks {
		t.Errorf("checks handoff = %q %q", label, prompt)
	}
	if !strings.HasSuffix(prompt, "…and 2 more failing checks not listed.") {
		t.Errorf("omitted tail missing: %q", prompt)
	}
	d.CheckRuns = nil
	if _, prompt, _ = pullHandoff(d); !strings.Contains(prompt, "1 check is red") {
		t.Errorf("unnamed prompt = %q", prompt)
	}
	if _, _, ok := pullHandoff(detail("OPEN", "MERGEABLE", "CLEAN", "")); ok {
		t.Error("a healthy PR has nothing to hand off")
	}
	if _, _, ok := pullHandoff(detail("MERGED", "CONFLICTING", "DIRTY", "")); ok {
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
	d := lichclient.PRDetail{Number: 7, Title: "feat: x", Body: "why", HeadRefName: "feat/x"}
	if got := mergeEditFor("squash", d); got.subject != "feat: x (#7)" || got.body != "why" || got.title != mergeEditSquashTitle {
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
