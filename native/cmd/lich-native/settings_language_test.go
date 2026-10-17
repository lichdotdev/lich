package main

import (
	"context"
	"testing"

	"github.com/omartelo/lich/native/i18n"
)

func spanish() i18n.Locale { return i18n.ES }

// Nothing stored: the interface follows the system and the prompts are in
// English, as the backend reads an absent prompt.language.
func TestLanguagesFollowTheSystemUntilChosen(t *testing.T) {
	_, client := newSidebarBackend(t)
	iface, prompt, err := loadLanguages(context.Background(), client, spanish)
	if err != nil {
		t.Fatal(err)
	}
	if iface != i18n.ES || prompt != i18n.EN {
		t.Errorf("interface %s, prompt %s", iface, prompt)
	}
}

// A stored choice beats the system, each language read from its own key; a
// tag of another region reads its language.
func TestStoredLanguagesBeatTheSystem(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.settings[uiLanguageKey+"@"], f.settings[promptLanguageKey+"@"] = "pt-PT", "zh-CN"
	iface, prompt, err := loadLanguages(context.Background(), client, spanish)
	if err != nil {
		t.Fatal(err)
	}
	if iface != i18n.PtBR || prompt != i18n.ZhCN {
		t.Errorf("interface %s, prompt %s", iface, prompt)
	}
}

// The agent prompts the window types are written in the prompt language,
// whatever the interface shows.
func TestPromptsFollowThePromptLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.ZhCN)
	i18n.SetPromptLanguage(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	defer i18n.SetPromptLanguage(i18n.EN)
	got := composeReviewComments([]reviewComment{{txt: "x"}}, "")
	want := bracketedPaste(i18n.In(i18n.PtBR, "prompts.reviewComments.header") + "\n\n- x")
	if got != want {
		t.Errorf("composeReviewComments = %q, want %q", got, want)
	}
}

// A language in the General section is searched by its translated title.
func TestTheLanguageRowsAreSearchedInTheInterfaceLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	hits := searchSettings("idioma dos")
	if len(hits) == 0 || hits[0].title != "Idioma dos prompts" || hits[0].section != sectionGeneral || hits[0].group != "Idioma" {
		t.Errorf("hits %+v", hits)
	}
}
