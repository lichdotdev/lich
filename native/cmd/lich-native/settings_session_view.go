package main

import (
	"log"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// sessionViewKey is the global setting a new session is drawn by
// (store.SessionViewKey): api.ViewChat, the default, or api.ViewTerminal.
const sessionViewKey = "session.view"

// sessionViewEntry is the row's place in the search, on the Providers page.
var sessionViewEntry = settingEntry{titleKey: "settings.sessionViewSetting.title", alsoKey: "settings.sessionViewSetting.searchWords", key: sessionViewKey}

// sessionViewSetting is the row's choice, as stored.
type sessionViewSetting struct {
	value  string
	choice ui.Enum
}

// sessionViewOf reads the stored choice: the chat unless the terminal was
// asked for.
func sessionViewOf(stored string) string {
	if stored == api.ViewTerminal {
		return api.ViewTerminal
	}
	return api.ViewChat
}

// loadSessionView reads the view new sessions open in.
func (s *settingsScreen) loadSessionView() error {
	raw, err := s.client.GetSetting(s.ctx, sessionViewKey, "")
	s.sessionView.value = sessionViewOf(raw)
	return err
}

// sessionViewRow is the view new sessions open in: Chat or Terminal, global.
func (s *settingsScreen) sessionViewRow(gtx C, th *ui.Theme, values scopedValues, first bool) D {
	r := s.providerRow(gtx, sessionViewEntry, values)
	r.description = i18n.T("settings.sessionViewSetting.description")
	g := &s.sessionView.choice
	if g.Update(gtx) {
		s.sessionView.value = sessionViewOf(g.Value)
		value := s.sessionView.value
		go func() {
			if err := s.client.SetSetting(s.ctx, sessionViewKey, "", value); err != nil {
				log.Printf("settings: save %s: %v", sessionViewKey, err)
			}
		}()
	}
	g.Value = s.sessionView.value
	return r.layout(gtx, th, first, func(gtx C, th *ui.Theme) D {
		return th.ToggleGroup(g,
			ui.ToggleItem{Value: api.ViewChat, Text: i18n.T("settings.sessionViewSetting.chat")},
			ui.ToggleItem{Value: api.ViewTerminal, Text: i18n.T("settings.sessionViewSetting.terminal")},
		).Layout(gtx)
	})
}
