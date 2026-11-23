package agentplugin

import (
	"strings"
	"testing"
)

// From EditNoteRelease, crushrc runs the touched report after the tool on a
// Crush that has PostToolUse (v0.99.0); an older plugin, an older Crush and one
// whose version could not be read keep it before the tool.
func TestCrushTouchedRidesPostToolUseFromTheEditNoteRelease(t *testing.T) {
	post := "hook add PostToolUse --name lich-touched"
	for _, tc := range []struct {
		plugin, crush string
		want          bool
	}{
		{EditNoteRelease, "0.99.0", true},
		{EditNoteRelease, "0.100.0", true},
		{EditNoteRelease, "0.98.2", false},
		{EditNoteRelease, "", false},
		{preVetoRelease, "0.100.0", false},
	} {
		rc := crushrcBlock(tc.plugin, tc.crush, "/opt/lich/hooks", "")
		if got := strings.Contains(rc, post); got != tc.want {
			t.Errorf("plugin %s on Crush %q: PostToolUse %v, want %v\n%s", tc.plugin, tc.crush, got, tc.want, rc)
		}
		// The flip moves the touched report alone: the session id stays where
		// that Crush's block puts it (crushStateHooks from crushEventsVersion).
		start := "hook add PreToolUse --name lich-session-id "
		if crushHasEvents(tc.crush) {
			start = "hook add SessionStart --name lich-session-id "
		}
		if !strings.Contains(rc, start) {
			t.Errorf("plugin %s on Crush %q: the session-start report moved\n%s", tc.plugin, tc.crush, rc)
		}
		if n := strings.Count(rc, "--name lich-touched "); n != 1 {
			t.Errorf("plugin %s on Crush %q: %d touched registrations, want 1\n%s", tc.plugin, tc.crush, n, rc)
		}
	}
}

// From EditNoteRelease, Kiro's touched report holds its note and the next
// prompt collects it; an older release registers neither argument.
func TestKiroTouchedHoldsAndThePromptCollects(t *testing.T) {
	for version, want := range map[string]bool{EditNoteRelease: true, preVetoRelease: false} {
		hooks := decodeKiroAgent(t, version, "/opt/lich/hooks")["hooks"].(map[string]any)
		holds := 0
		for _, entry := range hooks["postToolUse"].([]any) {
			if entry.(map[string]any)["command"] == kiroCommand("/opt/lich/hooks", kiroTouchedScript, "hold") {
				holds++
			}
		}
		collects := false
		for _, entry := range hooks["userPromptSubmit"].([]any) {
			collects = collects || entry.(map[string]any)["command"] == kiroCommand("/opt/lich/hooks", kiroTouchedScript, "collect")
		}
		if (holds == len(kiroWriteTools)) != want || collects != want {
			t.Errorf("%s: %d holding touched reports and collect %v, want both %v", version, holds, collects, want)
		}
	}
}
