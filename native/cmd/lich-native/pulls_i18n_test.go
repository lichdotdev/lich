package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

func testTheme() *ui.Theme { return &ui.Theme{Palette: ui.Dark} }

func TestPatternClauseNamesEveryOperatorAndNegation(t *testing.T) {
	cases := []struct {
		operator string
		negate   bool
		want     string
	}{
		{"regex", false, `message must match "x"`},
		{"regex", true, `message must not match "x"`},
		{"starts_with", false, `message must start with "x"`},
		{"starts_with", true, `message must not start with "x"`},
		{"ends_with", false, `message must end with "x"`},
		{"ends_with", true, `message must not end with "x"`},
		{"contains", false, `message must contain "x"`},
		{"contains", true, `message must not contain "x"`},
		{"never_seen", false, `message must match "x"`},
	}
	for _, c := range cases {
		p := lichclient.CommitPattern{Target: "message", Operator: c.operator, Pattern: "x", Negate: c.negate}
		if got := patternClause(p); got != c.want {
			t.Errorf("%s negate=%v: %q, want %q", c.operator, c.negate, got, c.want)
		}
	}
}

func TestPatternTargetKeepsAWordTheBackendAddsLater(t *testing.T) {
	if got := patternTarget("author email"); got != "author email" {
		t.Errorf("author email = %q", got)
	}
	if got := patternTarget("branch name"); got != "branch name" {
		t.Errorf("an unknown target = %q, want it as given", got)
	}
}

func TestMergeGateSpeaksTheInterfaceLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	rules := &lichclient.BranchRules{CommitPatterns: []lichclient.CommitPattern{{Target: "author email", Operator: "ends_with", Pattern: "@x.dev", Negate: true}}}
	blocked := detail("OPEN", "MERGEABLE", "BLOCKED", "APPROVED")
	want := `Um ruleset governa main: em todo commit, o e-mail do autor não deve terminar com "@x.dev".`
	if got := mergeHint(blocked, rules); got != want {
		t.Errorf("rule note = %q, want %q", got, want)
	}
	if got := mergeBlockedReason(detail("OPEN", "CONFLICTING", "BLOCKED", "")); got != "Conflita com main" {
		t.Errorf("conflicts = %q", got)
	}
}

func TestMergeHintCountsTheRedChecksInTheInterfaceLanguage(t *testing.T) {
	red := detail("OPEN", "MERGEABLE", "UNSTABLE", "")
	red.Checks.Failed = 1
	if got := mergeHint(red, nil); got != "1 check is failing; GitHub will merge over it" {
		t.Errorf("one = %q", got)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := mergeHint(red, nil); got != "1 verificação está falhando; o GitHub fará o merge mesmo assim" {
		t.Errorf("pt-BR one = %q", got)
	}
	red.Checks.Failed = 3
	if got := mergeHint(red, nil); got != "3 verificações estão falhando; o GitHub fará o merge mesmo assim" {
		t.Errorf("pt-BR other = %q", got)
	}
}

func TestChecksStatPicksThePluralOfTheInterfaceLanguage(t *testing.T) {
	one := lichclient.ChecksRollup{Total: 1, Passed: 1}
	many := lichclient.ChecksRollup{Total: 4, Passed: 4}
	if _, text, _, _ := checksStat(testTheme(), one); text != "1 check passed" {
		t.Errorf("one = %q", text)
	}
	if _, text, _, _ := checksStat(testTheme(), many); text != "4 checks passed" {
		t.Errorf("other = %q", text)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if _, text, _, _ := checksStat(testTheme(), one); text != "1 verificação passou" {
		t.Errorf("pt-BR one = %q", text)
	}
	if _, text, _, _ := checksStat(testTheme(), many); text != "4 verificações passaram" {
		t.Errorf("pt-BR other = %q", text)
	}
}

func TestCheckDurationSpeaksTheInterfaceLanguage(t *testing.T) {
	now := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := checkDuration("2026-01-01T00:50:00Z", "2026-01-01T00:52:05Z", now); got != "2min 5s" {
		t.Errorf("pt-BR minutes = %q", got)
	}
	i18n.SetCurrent(i18n.ZhCN)
	if got := checkDuration("2025-12-31T22:30:00Z", "2026-01-01T00:00:00Z", now); got != "1小时 30分钟" {
		t.Errorf("zh-CN hours = %q", got)
	}
	i18n.SetCurrent(i18n.KO)
	if got := checkDuration("2025-12-31T22:30:00Z", "2026-01-01T00:00:00Z", now); got != "1시간 30분" {
		t.Errorf("ko hours = %q", got)
	}
}

func TestLineAnchorNamesOneLineOrASpan(t *testing.T) {
	if got := lineAnchor(0, 7); got != "line 7" {
		t.Errorf("one line = %q", got)
	}
	if got := lineAnchor(3, 7); got != "lines 3-7" {
		t.Errorf("span = %q", got)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := lineAnchor(3, 7); got != "linhas 3-7" {
		t.Errorf("pt-BR span = %q", got)
	}
}

func TestReviewVerdictLeavesAStateItHasNoWordsForToTheCaller(t *testing.T) {
	if got, ok := reviewVerdict("CHANGES_REQUESTED"); !ok || got != "requested changes" {
		t.Errorf("changes requested = %q, %v", got, ok)
	}
	if _, ok := reviewVerdict("PENDING"); ok {
		t.Error("a state GitHub added later should not read as a known verdict")
	}
}

func TestSubmitVerdictTitlesFollowTheVerdict(t *testing.T) {
	for verdict, want := range map[string]string{"comment": "Comment", "approve": "Approve", "request_changes": "Request changes"} {
		if got := submitVerdictTitle(verdict); got != want {
			t.Errorf("%s = %q, want %q", verdict, got, want)
		}
	}
}

func TestRunningRemovalNamesTheWorktreeAndCountsTheSessions(t *testing.T) {
	one := i18n.T("pulls.pullsActions.running", i18n.Args{"count": 1, "name": "wt"})
	if one != "A session in wt is mid-turn. Removing the worktree stops it; the turn in flight is lost." {
		t.Errorf("one = %q", one)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	many := i18n.T("pulls.pullsActions.running", i18n.Args{"count": 3, "name": "wt"})
	if many != "3 sessões em wt estão no meio de um turno. Remover a worktree as interrompe; o turno em andamento é perdido." {
		t.Errorf("pt-BR other = %q", many)
	}
}

func TestPullsStateAndFilterLabelsFollowTheInterfaceLanguage(t *testing.T) {
	if got := pullsMerged.label(); got != "Merged" {
		t.Errorf("state = %q", got)
	}
	if got := filterFailing.label(); got != "Failing" {
		t.Errorf("filter = %q", got)
	}
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	if got := pullsClosed.none(); got != "Nenhum pull request fechado." {
		t.Errorf("pt-BR none = %q", got)
	}
	if got := filterDrafts.label(); got != "Rascunhos" {
		t.Errorf("pt-BR filter = %q", got)
	}
}
