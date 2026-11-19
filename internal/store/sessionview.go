package store

import (
	"context"
	"log/slog"

	"github.com/lichdotdev/lich/api"
)

// SessionViewKey is the global setting a new session is drawn by, api.ViewChat
// or api.ViewTerminal; absent reads as the chat.
const SessionViewKey = "session.view"

// SetChatKinds registers which providers can be drawn as a chat
// (internal/chat.Kinds, which imports this package). Startup wiring; without
// it every session is born on its terminal, which is what every test that does
// not wire it gets.
func (s *Service) SetChatKinds(fn func(kind string) bool) {
	s.chatKinds = fn
}

// bornView is how a new session of kind is drawn: as a chat when its provider
// has one and SessionViewKey does not ask for the terminal, "" (the terminal)
// otherwise. Read at the insert only: a session keeps the view it has, and a
// restored one the view it had.
func (s *Service) bornView(kind string) string {
	if s.chatKinds == nil || !s.chatKinds(kind) {
		return ""
	}
	value, err := s.GetSetting(context.Background(), api.GetSettingOptions{Key: SessionViewKey, ProjectID: globalScope})
	if err != nil {
		slog.Warn("read the view new sessions are drawn by", "err", err)
	}
	if value == api.ViewTerminal {
		return ""
	}
	return api.ViewChat
}
