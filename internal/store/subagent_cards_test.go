package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// Subagent cards are on unless the user turned them off, so only the literal
// "false" reads as off and every other value, none included, keeps them on.
func TestSubagentCardsAreOffOnlyForTheLiteralFalse(t *testing.T) {
	svc := newTestStore(t)
	if !svc.SubagentCards(providers.Claude) {
		t.Error("unconfigured provider has subagent cards off, want on")
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.subagentCards", ProjectID: globalScope, Value: "false"})
	if svc.SubagentCards(providers.Claude) {
		t.Error(`"false" leaves subagent cards on, want off`)
	}
	if !svc.SubagentCards(providers.Codex) {
		t.Error("another provider follows Claude's setting, want its own")
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.subagentCards", ProjectID: globalScope, Value: "true"})
	if !svc.SubagentCards(providers.Claude) {
		t.Error(`"true" leaves subagent cards off, want on`)
	}
}
