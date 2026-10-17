package store

import (
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
)

func TestPromptLanguageIsEnglishUntilSetAndPersists(t *testing.T) {
	svc := newTestStore(t)

	if got := svc.PromptLanguage(); got != prompt.English {
		t.Errorf("PromptLanguage = %q on a fresh workspace, want en", got)
	}
	if err := svc.SetSetting("prompt.language", "", "pt-BR"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.PortugueseBR {
		t.Errorf("PromptLanguage = %q after choosing pt-BR", got)
	}
	if err := svc.SetSetting("prompt.language", "", "klingon"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.English {
		t.Errorf("PromptLanguage = %q for an unknown tag, want en", got)
	}
}

func TestPromptLanguageAcceptsSpanish(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SetSetting("prompt.language", "", "es"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Spanish {
		t.Errorf("PromptLanguage = %q after choosing es", got)
	}
}

func TestPromptLanguageAcceptsJapanese(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SetSetting("prompt.language", "", "ja"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Japanese {
		t.Errorf("PromptLanguage = %q after choosing ja", got)
	}
}

func TestPromptLanguageAcceptsKorean(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SetSetting("prompt.language", "", "ko"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Korean {
		t.Errorf("PromptLanguage = %q after choosing ko", got)
	}
}

func TestPromptLanguageAcceptsRussian(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SetSetting("prompt.language", "", "ru"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.PromptLanguage(); got != prompt.Russian {
		t.Errorf("PromptLanguage = %q after choosing ru", got)
	}
}
