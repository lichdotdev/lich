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

// Every Claude Code session loads lich's release, whatever profile it runs in,
// so a worker opens cards by that release alone.
func TestClaudeNestsSubagentCardsFollowsTheRelease(t *testing.T) {
	root := bundleHome(t)
	if ClaudeNestsSubagentCards("") {
		t.Error("no release unpacked: want cards not to nest")
	}
	plantRelease(t, root, "0.18.4")
	if ClaudeNestsSubagentCards(t.TempDir()) {
		t.Error("on 0.18.4: want cards not to nest")
	}
	plantRelease(t, root, "0.19.0")
	if !ClaudeNestsSubagentCards("") || !ClaudeNestsSubagentCards(t.TempDir()) {
		t.Error("on 0.19.0: want cards to nest in Personal and in a profile alike")
	}
}

func TestAsksSubagentHookFromTheHookRelease(t *testing.T) {
	for version, want := range map[string]bool{"0.21.9": false, "0.22.0": true, "0.23.1": true, "": false} {
		if got := asksSubagentHook(version); got != want {
			t.Errorf("asksSubagentHook(%q) = %v, want %v", version, got, want)
		}
	}
}
