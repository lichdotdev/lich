package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

func TestPromptLanguageIsEnglishUntilSetAndPersists(t *testing.T) {
	svc := newTestStore(t)

	if got := svc.PromptLanguage(); got != prompt.English {
		t.Errorf("PromptLanguage = %q on a fresh workspace, want en", got)
	}
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "prompt.language", Value: "pt-BR"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.PortugueseBR {
		t.Errorf("PromptLanguage = %q after choosing pt-BR", got)
	}
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "prompt.language", Value: "klingon"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.English {
		t.Errorf("PromptLanguage = %q for an unknown tag, want en", got)
	}
}

func TestPromptLanguageAcceptsSpanish(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "prompt.language", Value: "es"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Spanish {
		t.Errorf("PromptLanguage = %q after choosing es", got)
	}
}

func TestPromptLanguageAcceptsJapanese(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "prompt.language", Value: "ja"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Japanese {
		t.Errorf("PromptLanguage = %q after choosing ja", got)
	}
}

func TestPromptLanguageAcceptsKorean(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "prompt.language", Value: "ko"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Korean {
		t.Errorf("PromptLanguage = %q after choosing ko", got)
	}
}

func TestPromptLanguageAcceptsRussian(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "prompt.language", Value: "ru"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Russian {
		t.Errorf("PromptLanguage = %q after choosing ru", got)
	}
}
