package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
)

// ponytail: English until the native window has i18n (pulls.*).
const (
	pullsMergedText    = "Pull request is merged"
	pullsClosedText    = "Pull request is closed"
	pullsDraftText     = "Pull request is a draft"
	pullsConflictsText = "Conflicts with %s"
	pullsBehindText    = "Base branch has moved — update this branch first"
	pullsChangesText   = "A reviewer asked for changes"
	pullsReviewReqText = "GitHub requires a review that has not been left"
	pullsRuleNoteText  = "A ruleset governs %s: every commit %s."
	pullsFailingOne    = "1 check is failing; GitHub will merge over it"
	pullsFailingMany   = "%d checks are failing; GitHub will merge over them"
)

// pullsState is which pull requests gh is asked for.
type pullsState string

const (
	pullsOpen   pullsState = "open"
	pullsClosed pullsState = "closed"
	pullsMerged pullsState = "merged"
)

var pullsStates = []pullsState{pullsOpen, pullsClosed, pullsMerged}

// pullsFilter is a quick filter over the rows gh answered.
type pullsFilter int

const (
	filterAll pullsFilter = iota
	filterReady
	filterDrafts
	filterFailing
	pullsFilterCount
)

var pullsFilterLabels = [pullsFilterCount]string{"All", "Ready", "Drafts", "Failing"}

type checkVerdict int

const (
	verdictNone checkVerdict = iota
	verdictPassed
	verdictPending
	verdictFailed
)

func verdictOf(c lichclient.ChecksRollup) checkVerdict {
	switch {
	case c.Total == 0:
		return verdictNone
	case c.Failed > 0:
		return verdictFailed
	case c.Pending > 0:
		return verdictPending
	}
	return verdictPassed
}

func (f pullsFilter) matches(pr lichclient.PRSummary) bool {
	switch f {
	case filterReady:
		return !pr.IsDraft
	case filterDrafts:
		return pr.IsDraft
	case filterFailing:
		return verdictOf(pr.Checks) == verdictFailed
	}
	return true
}

// matchesText is pull-request-list.ts matchesText: the number with or without
// its "#", the title, the author and the branch.
func matchesText(pr lichclient.PRSummary, text string) bool {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return true
	}
	n := strconv.Itoa(pr.Number)
	for _, field := range []string{"#" + n, n, pr.Title, pr.Author, pr.HeadRefName} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

// filterPulls is the rows the filter and the text keep, newest update first.
func filterPulls(list []lichclient.PRSummary, f pullsFilter, text string) []lichclient.PRSummary {
	var rows []lichclient.PRSummary
	for _, pr := range list {
		if f.matches(pr) && matchesText(pr, text) {
			rows = append(rows, pr)
		}
	}
	slices.SortStableFunc(rows, func(a, b lichclient.PRSummary) int { return strings.Compare(b.UpdatedAt, a.UpdatedAt) })
	return rows
}

// filterCounts is how many rows each quick filter would keep under text.
func filterCounts(list []lichclient.PRSummary, text string) [pullsFilterCount]int {
	var counts [pullsFilterCount]int
	for _, pr := range list {
		if !matchesText(pr, text) {
			continue
		}
		for f := range counts {
			if pullsFilter(f).matches(pr) {
				counts[f]++
			}
		}
	}
	return counts
}

// updatedAgo is the row's age; "" for a timestamp gh did not give.
func updatedAgo(iso string, now time.Time) string {
	at, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	return agoLabel(at.Unix(), now)
}

var everyMergeMethod = []string{"squash", "merge", "rebase"}

var mergeMethodLabels = map[string]string{"squash": "Squash and merge", "merge": "Create a merge commit", "rebase": "Rebase and merge"}

// allowedMergeMethods is merge-gate.ts: what the base's rules name, all three
// when they name none.
func allowedMergeMethods(rules *lichclient.BranchRules) []string {
	var allowed []string
	for _, m := range everyMergeMethod {
		if rules != nil && slices.Contains(rules.AllowedMergeMethods, m) {
			allowed = append(allowed, m)
		}
	}
	if len(allowed) == 0 {
		return everyMergeMethod
	}
	return allowed
}

func conflictsWithBase(d lichclient.PRDetail) bool {
	return d.MergeStateStatus == "DIRTY" || d.Mergeable == "CONFLICTING"
}

// mergeBlockedReason is merge-gate.ts: why the merge cannot go, "" when GitHub
// would take it (or a BLOCKED it gives no reason for, which only the merge
// attempt can explain).
func mergeBlockedReason(d lichclient.PRDetail) string {
	switch {
	case d.State == "MERGED":
		return pullsMergedText
	case d.State != "OPEN":
		return pullsClosedText
	case d.IsDraft:
		return pullsDraftText
	case conflictsWithBase(d):
		return fmt.Sprintf(pullsConflictsText, d.BaseRefName)
	case d.MergeStateStatus == "BEHIND":
		return pullsBehindText
	case d.MergeStateStatus != "BLOCKED":
		return ""
	case d.ReviewDecision == "CHANGES_REQUESTED":
		return pullsChangesText
	case d.ReviewDecision == "REVIEW_REQUIRED":
		return pullsReviewReqText
	}
	return ""
}

// canAdminOverride is merge-gate.ts: an administrator may merge past BLOCKED
// or BEHIND, never past a conflict.
func canAdminOverride(d lichclient.PRDetail, rules *lichclient.BranchRules) bool {
	if rules == nil || !rules.ViewerCanBypass || d.State != "OPEN" || d.IsDraft || d.Mergeable == "CONFLICTING" {
		return false
	}
	return d.MergeStateStatus == "BLOCKED" || d.MergeStateStatus == "BEHIND"
}

var patternVerbs = map[string][2]string{
	"regex":       {"match", "not match"},
	"starts_with": {"start with", "not start with"},
	"ends_with":   {"end with", "not end with"},
	"contains":    {"contain", "not contain"},
}

// mergeRuleNote is merge-gate.ts: on BLOCKED, the commit patterns the base's
// rules hold every commit to, the one rule nothing else on the PR shows.
func mergeRuleNote(d lichclient.PRDetail, rules *lichclient.BranchRules) string {
	if d.MergeStateStatus != "BLOCKED" || rules == nil || len(rules.CommitPatterns) == 0 {
		return ""
	}
	clauses := make([]string, len(rules.CommitPatterns))
	for i, p := range rules.CommitPatterns {
		verbs, ok := patternVerbs[p.Operator]
		if !ok {
			verbs = patternVerbs["regex"]
		}
		verb := verbs[0]
		if p.Negate {
			verb = verbs[1]
		}
		clauses[i] = fmt.Sprintf("%s must %s %q", p.Target, verb, p.Pattern)
	}
	return fmt.Sprintf(pullsRuleNoteText, d.BaseRefName, strings.Join(clauses, "; "))
}

// mergeHint is what the Merge button's tooltip says: the block, else the rule
// note, else the red checks GitHub would merge over.
func mergeHint(d lichclient.PRDetail, rules *lichclient.BranchRules) string {
	if r := mergeBlockedReason(d); r != "" {
		return r
	}
	if n := mergeRuleNote(d, rules); n != "" {
		return n
	}
	switch d.Checks.Failed {
	case 0:
		return ""
	case 1:
		return pullsFailingOne
	}
	return fmt.Sprintf(pullsFailingMany, d.Checks.Failed)
}

// handoffNamedChecks is pr-handoff.ts NAMED_CHECKS: past it the prompt counts.
const handoffNamedChecks = 8

// pullHandoff is pr-handoff.ts: what is wrong with an open PR, conflicts
// first, as a button label and the prompt that hands it over; ok false when
// nothing is.
func pullHandoff(d lichclient.PRDetail) (label, prompt string, ok bool) {
	if d.State != "OPEN" {
		return "", "", false
	}
	if conflictsWithBase(d) {
		return "Resolve conflicts", i18n.Prompt("prompts.pullRequest.conflicts",
			i18n.Args{"number": d.Number, "branch": d.HeadRefName, "base": d.BaseRefName}), true
	}
	if d.Checks.Failed > 0 {
		return "Fix CI errors", checksPrompt(d), true
	}
	return "", "", false
}

func checksPrompt(d lichclient.PRDetail) string {
	var failed []lichclient.CheckItem
	for _, c := range d.CheckRuns {
		if c.State == "failed" {
			failed = append(failed, c)
		}
	}
	if len(failed) == 0 {
		return i18n.Prompt("prompts.pullRequest.checksUnnamed",
			i18n.Args{"number": d.Number, "branch": d.HeadRefName, "count": d.Checks.Failed})
	}
	named := failed[:min(len(failed), handoffNamedChecks)]
	lines := make([]string, len(named))
	for i, c := range named {
		lines[i] = "- " + c.Name
		if c.URL != "" {
			lines[i] += " — " + c.URL
		}
	}
	out := i18n.Prompt("prompts.pullRequest.checksListed",
		i18n.Args{"number": d.Number, "branch": d.HeadRefName, "checks": strings.Join(lines, "\n")})
	if omitted := len(failed) - len(named); omitted > 0 {
		out += i18n.Prompt("prompts.pullRequest.checksOmitted", i18n.Args{"count": omitted})
	}
	return out
}

// checkDuration is check-duration.ts: how long a check ran, or has run while
// it still does; "" without a start.
func checkDuration(started, completed string, now time.Time) string {
	start, err := time.Parse(time.RFC3339, started)
	if err != nil {
		return ""
	}
	end := now
	if completed != "" {
		if end, err = time.Parse(time.RFC3339, completed); err != nil {
			return ""
		}
	}
	if end.Before(start) {
		return ""
	}
	total := int(end.Sub(start).Round(time.Second) / time.Second)
	switch {
	case total < 60:
		return fmt.Sprintf("%ds", total)
	case total < 3600:
		if total%60 == 0 {
			return fmt.Sprintf("%dm", total/60)
		}
		return fmt.Sprintf("%dm %ds", total/60, total%60)
	}
	if m := total / 60 % 60; m != 0 {
		return fmt.Sprintf("%dh %dm", total/3600, m)
	}
	return fmt.Sprintf("%dh", total/3600)
}

// commitMeta is the commit's second line: who and when.
func commitMeta(c lichclient.PRCommit, now time.Time) string {
	who := c.Login
	if who == "" {
		who = c.Name
	}
	at, err := time.Parse(time.RFC3339, c.Date)
	if err != nil {
		return who
	}
	if who == "" {
		return agoLabel(at.Unix(), now)
	}
	return who + " · " + agoLabel(at.Unix(), now)
}
