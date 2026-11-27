package terminal

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/subagent"
)

func TestUltracodeArgs(t *testing.T) {
	t.Parallel()
	if got := claudeSettingsArgs(providers.Claude, true, subagent.Card{}); !slices.Equal(got, []string{"--settings", `{"ultracode":true}`}) {
		t.Errorf("claude, on: args = %v, want --settings with the ultracode key", got)
	}
	if got := claudeSettingsArgs(providers.Claude, false, subagent.Card{}); got != nil {
		t.Errorf("claude, off: args = %v, want none", got)
	}
	for _, kind := range []string{
		providers.Codex, providers.Antigravity, providers.OpenCode, providers.OMP,
		providers.Crush, providers.Cursor, providers.Kiro, KindShell,
	} {
		if got := claudeSettingsArgs(kind, true, subagent.Card{}); got != nil {
			t.Errorf("%s, on: args = %v, want none — only Claude Code has an ultracode", kind, got)
		}
		if SupportsUltracode(kind) {
			t.Errorf("SupportsUltracode(%q) = true, want false", kind)
		}
	}
}
