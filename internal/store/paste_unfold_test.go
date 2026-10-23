package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// A long paste stays folded unless the user turned unfolding on, so only the
// literal "true" reads as on.
func TestPasteUnfoldIsOnOnlyForTheLiteralTrue(t *testing.T) {
	svc := newTestStore(t)
	if svc.PasteUnfold(providers.OpenCode) {
		t.Error("unconfigured provider unfolds pastes, want off")
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.opencode.pasteUnfold", ProjectID: globalScope, Value: "true"})
	if !svc.PasteUnfold(providers.OpenCode) {
		t.Error(`"true" leaves pastes folded, want unfolded`)
	}
	if svc.PasteUnfold(providers.Claude) {
		t.Error("another provider follows opencode's setting, want its own")
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.opencode.pasteUnfold", ProjectID: globalScope, Value: "false"})
	if svc.PasteUnfold(providers.OpenCode) {
		t.Error(`"false" leaves pastes unfolded, want folded`)
	}
}
