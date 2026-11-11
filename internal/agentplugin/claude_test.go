package agentplugin

import "testing"

// A worker opens cards only under a release that tells it a worker by its
// depth, since an older one would stop answering for it.
func TestNestsSubagentCardsFromTheFirstNestingRelease(t *testing.T) {
	cases := map[string]bool{
		"0.18.4":       false,
		"0.19.0-rc.1":  false,
		"0.19.0":       true,
		"0.19.2":       true,
		"not-a-semver": false,
	}
	for version, want := range cases {
		if got := nestsSubagentCards(version); got != want {
			t.Errorf("nestsSubagentCards(%q) = %v, want %v", version, got, want)
		}
	}
}

// The briefing points Claude Code at its own Agent tool only once the
// installed plugin is a release whose mod turns subagents into lich cards.
func TestRunsSubagentCardsFromTheFirstCardRelease(t *testing.T) {
	cases := map[string]bool{
		"0.14.0":       false,
		"0.15.0-rc.1":  false,
		"0.15.0":       true,
		"0.15.3":       true,
		"1.0.0":        true,
		"not-a-semver": false,
	}
	for version, want := range cases {
		if got := runsSubagentCards(version); got != want {
			t.Errorf("runsSubagentCards(%q) = %v, want %v", version, got, want)
		}
	}
}

// A profile has a plugin set of its own, so a worker may open cards only if the
// plugin in the profile the session runs in is a release that tells it by depth,
// whatever Personal's is.
func TestClaudeNestsSubagentCardsAnswersForTheProfile(t *testing.T) {
	state := func(version string) string {
		return `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"` + version + `"}]}}`
	}
	personal := writeClaudeState(t, state("0.19.0"))
	t.Setenv("CLAUDE_CONFIG_DIR", personal)
	older := writeClaudeState(t, state("0.18.4"))
	t.Setenv("CLAUDE_CONFIG_DIR", personal)

	if !ClaudeNestsSubagentCards("") {
		t.Error("Personal on 0.19.0: want cards to nest")
	}
	if ClaudeNestsSubagentCards(older) {
		t.Error("a profile on 0.18.4 beside a Personal on 0.19.0: want cards not to nest")
	}
	if ClaudeNestsSubagentCards(t.TempDir()) {
		t.Error("a profile with no plugin: want cards not to nest")
	}
}
