package main

import (
	"context"
	"log"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// The two languages, never derived from each other. The interface's is the
// web's localStorage key, kept as a global setting here; the web window
// writes its own choice to the same setting, so a language picked there is
// the one this window opens in. The prompt language is the backend's own
// setting (internal/store promptLanguageKey).
const (
	uiLanguageKey     = "lich.appearance.language"
	promptLanguageKey = "prompt.language"
)

// languageMenus are the two pickers' menus.
type languageMenus struct {
	ui, prompt ui.MenuState
}

// loadLanguages reads both languages: the interface's is the stored choice,
// or system's before anyone made one; the prompt language is English when
// absent, as the backend reads it.
func loadLanguages(ctx context.Context, c *lichclient.Client, system func() i18n.Locale) (iface, prompt i18n.Locale, err error) {
	stored, err := c.GetSetting(ctx, uiLanguageKey, "")
	if err != nil {
		return "", "", err
	}
	iface = system()
	if stored != "" {
		iface = i18n.Match(stored)
	}
	storedPrompt, err := c.GetSetting(ctx, promptLanguageKey, "")
	if err != nil {
		return "", "", err
	}
	return iface, i18n.Match(storedPrompt), nil
}

// loadLanguages sets both languages before the first frame.
func (u *window) loadLanguages() error {
	iface, prompt, err := loadLanguages(u.ctx, u.client, i18n.System)
	if err != nil {
		return err
	}
	i18n.SetCurrent(iface)
	i18n.SetPromptLanguage(prompt)
	return nil
}

// languageRow is LanguageSetting and PromptLanguageSetting: a select of the
// four languages, each named in itself.
func (s *settingsScreen) languageRow(gtx C, th *ui.Theme, key string, r settingRow, first bool) D {
	menu, current, choose := &s.languages.ui, i18n.Current(), s.chooseLanguage
	r.description = i18n.T("settings.language.uiDescription")
	if key == promptLanguageKey {
		menu, current, choose = &s.languages.prompt, i18n.PromptLanguage(), s.choosePromptLanguage
		r.description = i18n.T("settings.language.promptDescription")
	}
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		entries := make([]ui.MenuEntry, len(i18n.Locales))
		for i, l := range i18n.Locales {
			entries[i] = ui.MenuEntry{Kind: ui.MenuRadioItem, Label: l.Name(), Checked: l == current, OnClick: func() { choose(l) }}
		}
		return th.DropdownMenu(s.root, menu, entries).Layout(gtx, func(gtx C) D { return fontTrigger(gtx, th, current.Name()) })
	})
}

// chooseLanguage shows the interface in l from the next frame on and keeps
// the choice.
func (s *settingsScreen) chooseLanguage(l i18n.Locale) {
	i18n.SetCurrent(l)
	s.invalidate()
	go func() {
		if err := s.client.SetSetting(s.ctx, uiLanguageKey, "", string(l)); err != nil {
			log.Printf("settings: save %s: %v", uiLanguageKey, err)
		}
	}()
}

// choosePromptLanguage is setPromptLanguage: the choice shows at once, and
// the one before it comes back, with a toast, when the backend refuses it.
func (s *settingsScreen) choosePromptLanguage(l i18n.Locale) {
	previous := i18n.PromptLanguage()
	i18n.SetPromptLanguage(l)
	go func() {
		err := s.client.SetSetting(s.ctx, promptLanguageKey, "", string(l))
		if err == nil {
			return
		}
		if i18n.PromptLanguage() == l {
			i18n.SetPromptLanguage(previous)
		}
		title := i18n.T("settings.language.promptSaveFailed", i18n.Args{"error": lichclient.ErrorText(err)})
		s.theme.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: title})
		s.invalidate()
	}()
}
