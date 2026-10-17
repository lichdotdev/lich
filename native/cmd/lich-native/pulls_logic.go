package main

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
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

func (f pullsFilter) label() string {
	switch f {
	case filterReady:
		return i18n.T("pulls.pullsList.filter.ready")
	case filterDrafts:
		return i18n.T("pulls.pullsList.filter.drafts")
	case filterFailing:
		return i18n.T("pulls.pullsList.filter.failing")
	}
	return i18n.T("pulls.pullsList.filter.all")
}

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

func mergeMethodLabel(method string) string {
	switch method {
	case "squash":
		return i18n.T("pulls.mergeMessageDialog.squashAndMerge")
	case "merge":
		return i18n.T("pulls.mergeMessageDialog.createMergeCommit")
	}
	return i18n.T("pulls.pullRequestView.rebaseAndMerge")
}

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
		return i18n.T("pulls.mergeGate.merged")
	case d.State != "OPEN":
		return i18n.T("pulls.mergeGate.closed")
	case d.IsDraft:
		return i18n.T("pulls.mergeGate.draft")
	case conflictsWithBase(d):
		return i18n.T("pulls.mergeGate.conflicts", i18n.Args{"base": d.BaseRefName})
	case d.MergeStateStatus == "BEHIND":
		return i18n.T("pulls.mergeGate.behind")
	case d.MergeStateStatus != "BLOCKED":
		return ""
	case d.ReviewDecision == "CHANGES_REQUESTED":
		return i18n.T("pulls.mergeGate.changesRequested")
	case d.ReviewDecision == "REVIEW_REQUIRED":
		return i18n.T("pulls.mergeGate.reviewRequired")
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

// patternClause is "<target> must <verb> <pattern>" in the interface
// language. An operator this build has never seen reads as a match, the
// pattern beside it still saying what is asked.
func patternClause(p lichclient.CommitPattern) string {
	args := i18n.Args{"target": patternTarget(p.Target), "pattern": strconv.Quote(p.Pattern)}
	switch {
	case p.Operator == "starts_with" && p.Negate:
		return i18n.T("pulls.mergeGate.pattern.notStartWith", args)
	case p.Operator == "starts_with":
		return i18n.T("pulls.mergeGate.pattern.startWith", args)
	case p.Operator == "ends_with" && p.Negate:
		return i18n.T("pulls.mergeGate.pattern.notEndWith", args)
	case p.Operator == "ends_with":
		return i18n.T("pulls.mergeGate.pattern.endWith", args)
	case p.Operator == "contains" && p.Negate:
		return i18n.T("pulls.mergeGate.pattern.notContain", args)
	case p.Operator == "contains":
		return i18n.T("pulls.mergeGate.pattern.contain", args)
	case p.Negate:
		return i18n.T("pulls.mergeGate.pattern.notMatch", args)
	}
	return i18n.T("pulls.mergeGate.pattern.match", args)
}

// patternTarget is the word the backend names a commit field by
// (internal/project.commitPatternTargets), one it does not know left as is.
func patternTarget(target string) string {
	switch target {
	case "message":
		return i18n.T("pulls.mergeGate.target.message")
	case "author email":
		return i18n.T("pulls.mergeGate.target.authorEmail")
	case "committer email":
		return i18n.T("pulls.mergeGate.target.committerEmail")
	}
	return target
}

// mergeRuleNote is merge-gate.ts: on BLOCKED, the commit patterns the base's
// rules hold every commit to, the one rule nothing else on the PR shows.
func mergeRuleNote(d lichclient.PRDetail, rules *lichclient.BranchRules) string {
	if d.MergeStateStatus != "BLOCKED" || rules == nil || len(rules.CommitPatterns) == 0 {
		return ""
	}
	clauses := make([]string, len(rules.CommitPatterns))
	for i, p := range rules.CommitPatterns {
		clauses[i] = patternClause(p)
	}
	return i18n.T("pulls.mergeGate.ruleNote", i18n.Args{"base": d.BaseRefName, "clauses": strings.Join(clauses, "; ")})
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
	if d.Checks.Failed == 0 {
		return ""
	}
	return i18n.T("pulls.pullsLogic.failing", i18n.Args{"count": d.Checks.Failed})
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
		return i18n.T("pulls.prHandoff.resolveConflicts"), i18n.Prompt("prompts.pullRequest.conflicts",
			i18n.Args{"number": d.Number, "branch": d.HeadRefName, "base": d.BaseRefName}), true
	}
	if d.Checks.Failed > 0 {
		return i18n.T("pulls.prHandoff.fixChecks"), checksPrompt(d), true
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
		return durationUnit("second", total)
	case total < 3600:
		minutes := durationUnit("minute", total/60)
		if total%60 == 0 {
			return minutes
		}
		return minutes + " " + durationUnit("second", total%60)
	}
	hours := durationUnit("hour", total/3600)
	if m := total / 60 % 60; m != 0 {
		return hours + " " + durationUnit("minute", m)
	}
	return hours
}

// durationUnit is value of unit ("second", "minute" or "hour") in the
// interface language's narrow form.
func durationUnit(unit string, value int) string {
	args := i18n.Args{"value": value}
	switch unit {
	case "second":
		return i18n.T("pulls.pullsLogic.durationSeconds", args)
	case "minute":
		return i18n.T("pulls.pullsLogic.durationMinutes", args)
	}
	return i18n.T("pulls.pullsLogic.durationHours", args)
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
