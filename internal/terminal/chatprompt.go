package terminal

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
)

// ChatSink is the chat service (internal/chat) as the relay reaches a session
// drawn as a chat: its process has no terminal to type at and no mod, so a
// prompt goes to the chat as its next message.
type ChatSink interface {
	// Opened is whether session id's chat has opened its conversation, before
	// which a message would land mid-handshake.
	Opened(id string) bool
	// Take sends text as session id's next message, at once or after the turn
	// running, and waits on that turn's start the way a mod's ack is waited on.
	Take(id, text string) (relay.HandedPrompt, error)
}

// SetChat wires the chat service the relay's prompts reach a chat through.
func (s *Service) SetChat(c ChatSink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chat = c
}

// chatSink is the chat service when session id runs as a chat, nil otherwise.
// Chat is whether session id is drawn as a chat (relay.Terminal).
func (s *Service) Chat(id string) bool { return s.chatSink(id) != nil }

func (s *Service) chatSink(id string) ChatSink {
	if !s.spawnOf(id).chat {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chat
}

// Reachable answers whether session id has a process a message can reach:
// its terminal, or its chat. Live stays the terminal's question, since a write
// that needs the PTY must not be told a chat is there to take it.
func (s *Service) Reachable(_ context.Context, opts api.SessionIDOptions) (bool, error) {
	return s.live(opts.ID) || s.spawnOf(opts.ID).chat, nil
}
