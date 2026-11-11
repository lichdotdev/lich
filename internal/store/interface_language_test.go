package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestInterfaceLanguageIsTheStoredChoiceOrTheEnvironments(t *testing.T) {
	svc := newTestStore(t)
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "ja_JP.UTF-8")
	if got := svc.InterfaceLanguage(); got != "ja" {
		t.Errorf("unset under LANG=ja_JP = %q, want ja", got)
	}
	t.Setenv("LANG", "fr_FR.UTF-8")
	if got := svc.InterfaceLanguage(); got != "en" {
		t.Errorf("unset under a language lich lacks = %q, want en", got)
	}
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: interfaceLanguageKey, Value: "pt-br"}); err != nil {
		t.Fatal(err)
	}
	if got := svc.InterfaceLanguage(); got != "pt-BR" {
		t.Errorf("stored pt-br = %q, want pt-BR", got)
	}
}
