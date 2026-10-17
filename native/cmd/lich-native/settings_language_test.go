package main

import (
	"context"
	"errors"
	"testing"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/ui"
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

func drawsNone(i18n.Locale) bool { return false }
func drawsAll(i18n.Locale) bool  { return true }

func drawsOnly(l i18n.Locale) func(i18n.Locale) bool {
	return func(asked i18n.Locale) bool { return asked == l }
}

func isCJK(l i18n.Locale) bool { return l == i18n.ZhCN || l == i18n.JA || l == i18n.KO }

func TestOnlyCJKLanguagesWaitForTheirFont(t *testing.T) {
	for _, l := range i18n.Locales {
		if got, want := needsHanFont(l, drawsNone), isCJK(l); got != want {
			t.Errorf("%s without a CJK font: waits %v, want %v", l, got, want)
		}
		if needsHanFont(l, drawsAll) {
			t.Errorf("%s waits with its font in place", l)
		}
		if needsHanFont(l, drawsOnly(l)) {
			t.Errorf("%s waits with its own script drawn", l)
		}
	}
}

// A face for one CJK script is no face for another: Chinese's draws no kana
// and no Hangul.
func TestAChineseFontLeavesJapaneseAndKoreanWaiting(t *testing.T) {
	for _, l := range []i18n.Locale{i18n.JA, i18n.KO} {
		if !needsHanFont(l, drawsOnly(i18n.ZhCN)) {
			t.Errorf("%s does not wait with only a Chinese font", l)
		}
	}
}

func TestALaunchInCJKWithoutAFontOpensInEnglishAndOffersIt(t *testing.T) {
	for _, l := range []i18n.Locale{i18n.ZhCN, i18n.JA, i18n.KO} {
		if shown, offer := launchLanguage(l, drawsNone); shown != i18n.EN || !offer {
			t.Errorf("%s, no font: shown %s, offer %v", l, shown, offer)
		}
		if shown, offer := launchLanguage(l, drawsOnly(l)); shown != l || offer {
			t.Errorf("%s, with a font: shown %s, offer %v", l, shown, offer)
		}
	}
	for _, l := range []i18n.Locale{i18n.PtBR, i18n.RU} {
		if shown, offer := launchLanguage(l, drawsNone); shown != l || offer {
			t.Errorf("%s: shown %s, offer %v", l, shown, offer)
		}
	}
}

func languageScreen(t *testing.T) *settingsScreen {
	_, client := newSidebarBackend(t)
	s := newSettingsScreen(t.Context(), client, new(ui.Root), func() {})
	s.theme = &themePicker{toaster: new(ui.Toaster)}
	s.languages.draws = drawsNone
	i18n.SetCurrent(i18n.PtBR)
	t.Cleanup(func() { i18n.SetCurrent(i18n.EN) })
	return s
}

func TestChoosingChineseWithoutAFontAsksBeforeSwitching(t *testing.T) {
	s := languageScreen(t)
	s.chooseLanguage(i18n.ZhCN)
	if !s.languages.offer.open || s.languages.offer.target != i18n.ZhCN {
		t.Fatalf("offer open %v for %s", s.languages.offer.open, s.languages.offer.target)
	}
	if i18n.Current() != i18n.PtBR {
		t.Errorf("switched to %s before the font", i18n.Current())
	}
}

func TestChoosingJapaneseWithOnlyAChineseFontAsksForJapanese(t *testing.T) {
	s := languageScreen(t)
	s.languages.draws = drawsOnly(i18n.ZhCN)
	s.chooseLanguage(i18n.JA)
	if !s.languages.offer.open || s.languages.offer.target != i18n.JA || i18n.Current() != i18n.PtBR {
		t.Errorf("offer open %v for %s, current %s", s.languages.offer.open, s.languages.offer.target, i18n.Current())
	}
}

func TestChoosingChineseWithAFontSwitchesAtOnce(t *testing.T) {
	s := languageScreen(t)
	s.languages.draws = drawsAll
	s.chooseLanguage(i18n.ZhCN)
	if s.languages.offer.open || i18n.Current() != i18n.ZhCN {
		t.Errorf("open %v, current %s", s.languages.offer.open, i18n.Current())
	}
}

func TestADownloadSwitchesOnlyOnceItsFontLoads(t *testing.T) {
	s := languageScreen(t)
	var applied []i18n.Locale
	s.languages.reloadFonts = func() error { s.languages.draws = drawsOnly(i18n.KO); return nil }
	s.languages.offer.show(i18n.KO, func(l i18n.Locale) { applied = append(applied, l) })
	s.finishHanFontDownload(&hanFontOutcome{run: s.languages.offer.run})
	if len(applied) != 1 || applied[0] != i18n.KO || s.languages.offer.open {
		t.Errorf("applied %v, open %v", applied, s.languages.offer.open)
	}
}

func TestADownloadThatDoesNotDrawTheLanguageKeepsIt(t *testing.T) {
	s := languageScreen(t)
	s.languages.reloadFonts = func() error { return nil }
	s.languages.offer.show(i18n.JA, func(l i18n.Locale) { t.Errorf("switched to %s", l) })
	s.finishHanFontDownload(&hanFontOutcome{run: s.languages.offer.run})
	if s.languages.offer.open || i18n.Current() != i18n.PtBR {
		t.Errorf("open %v, current %s", s.languages.offer.open, i18n.Current())
	}
}

func TestAFailedOrCanceledDownloadKeepsTheLanguage(t *testing.T) {
	s := languageScreen(t)
	s.languages.reloadFonts = func() error { t.Error("reloaded the fonts"); return nil }
	apply := func(l i18n.Locale) { t.Errorf("switched to %s", l) }

	s.languages.offer.show(i18n.ZhCN, apply)
	s.finishHanFontDownload(&hanFontOutcome{run: s.languages.offer.run, err: errors.New("download NotoSansSC-Regular.otf: 404 Not Found")})
	if s.languages.offer.open {
		t.Error("the dialog stayed open after the failure")
	}

	s.languages.offer.show(i18n.ZhCN, apply)
	run := s.languages.offer.run
	s.languages.offer.dismiss()
	s.finishHanFontDownload(&hanFontOutcome{run: run})
	if i18n.Current() != i18n.PtBR {
		t.Errorf("current %s", i18n.Current())
	}
}

// The notes are read once and kept; a new interface language reads them
// again, or the dialog names its groups in one language and lists them in
// the other.
func TestChoosingALanguageRereadsThePatchNotes(t *testing.T) {
	s := languageScreen(t)
	s.updates.readNotes(s)
	s.keepLanguage(i18n.RU)
	eventually(t, "the notes in Russian", func() bool {
		s.updates.mu.Lock()
		defer s.updates.mu.Unlock()
		return s.updates.notes.Groups[0].Items[0] == string(i18n.RU)
	})
}
