package terminal

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
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
	// LastAnswer is the final message of session id's last turn, "" when that
	// turn wrote none.
	LastAnswer(id string) string
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

// FinalMessage is the final message of session id's last turn, whole, and
// whether lich can read one for it (relay.Terminal): a chat's driver has it;
// in a terminal it is the last-turn recap's reading, uncut, for every provider
// whose conversation lich reads (transcriptReaderFor, sessionDBTexts), which
// leaves Cursor CLI out.
func (s *Service) FinalMessage(id string) (string, bool) {
	if sink := s.chatSink(id); sink != nil {
		return sink.LastAnswer(id), true
	}
	if s.kindOf(id) == providers.Cursor {
		return "", false
	}
	providerSessionID, err := s.store.ProviderSession(id)
	if err != nil || providerSessionID == "" {
		return "", true
	}
	src, ok := usageSourceFor(providerSessionID, s.spawnOf(id).cwd)
	if !ok {
		return "", true
	}
	return s.recap.said(id, src), true
}

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
