package main

import (
	"slices"
	"testing"
)

func TestOnlyLooseningARungAsks(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
	}{
		{rungEverywhere, rungWorktrees, true},
		{rungWorktrees, rungAsk, true},
		{rungAsk, rungOff, true},
		{rungEverywhere, rungOff, true},
		{rungOff, rungEverywhere, false},
		{rungAsk, rungWorktrees, false},
		{rungAsk, rungAsk, false},
	}
	for _, c := range cases {
		if got := loosens(c.current, c.next); got != c.want {
			t.Errorf("%s → %s asks = %v, want %v", c.current, c.next, got, c.want)
		}
	}
}

func TestUnknownRungReadsAsOff(t *testing.T) {
	for raw, want := range map[string]string{"": rungOff, "sometimes": rungOff, rungWorktrees: rungWorktrees} {
		if got := sandboxLevel(raw); got != want {
			t.Errorf("sandboxLevel(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestAProjectRowShowsWhatItsSpawnReads(t *testing.T) {
	values := scopedValues{sshAgentKey: {"pg": "true"}}
	globals := map[string]string{sshAgentKey: "false"}
	cases := map[string]string{"": "false", "pg": "true", "lich": "false"}
	for scope, want := range cases {
		if got := effectiveValue(values, globals, sshAgentKey, scope); got != want {
			t.Errorf("from %q = %q, want %q", scope, got, want)
		}
	}
}

func TestGitHubTokenNamesTheAccount(t *testing.T) {
	cases := map[string]string{
		"":                    "As gh's active account.",
		"omartelo":            "As omartelo.",
		"github.com/omartelo": "As omartelo.",
	}
	for account, want := range cases {
		if got := ghAccountLine(account); got != want {
			t.Errorf("ghAccountLine(%q) = %q, want %q", account, got, want)
		}
	}
}

func TestAgentKeysLineSaysNothingLoaded(t *testing.T) {
	if got := agentKeysLine(nil); got != "Nothing loaded." {
		t.Errorf("no keys = %q", got)
	}
	if got := agentKeysLine([]string{"a", "b"}); got != "Loaded: a · b" {
		t.Errorf("two keys = %q", got)
	}
}

func TestEveryProviderRungIsScopedAndCounted(t *testing.T) {
	keys := scopedKeys()
	for _, kind := range []string{"claude", "codex", "antigravity", "opencode", "omp", "crush", "cursor", "kiro"} {
		if !slices.Contains(keys, "provider."+kind+".sandbox") {
			t.Errorf("%s's rung is not read per project", kind)
		}
	}
	if slices.Contains(keys, "provider.shell.sandbox") || slices.Contains(keys, sandboxLadderKey) {
		t.Error("a terminal rung or the ladder's row is read as a setting")
	}
	values := scopedValues{"provider.codex.sandbox": {"pg": rungEverywhere}, sshAgentKey: {"pg": "true"}}
	if got := sectionBadge(sectionByID(sectionSandbox), "pg", openTabs, values); got != 2 {
		t.Errorf("Sandbox badge in pg = %d, want the rung and the grant", got)
	}
}
