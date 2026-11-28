package terminal

import (
	"context"
	"log/slog"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// ChatSink is the chat service (internal/chat) as the relay reaches a session
// drawn as a chat: its process has no terminal to type at, so a prompt goes to
// the chat as its next message.
type ChatSink interface {
	// Open starts session id's chat on its conversation, unless it runs
	// already, and returns once the conversation is open.
	Open(ctx context.Context, id string) error
	// Opened is whether session id's chat has opened its conversation, before
	// which a message would land mid-handshake.
	Opened(id string) bool
	// Take sends text as session id's next message, at once or after the turn
	// running, and waits on that turn's start.
	Take(id, text string) (relay.HandedPrompt, error)
	// LastAnswer is the final message of session id's last turn, "" when that
	// turn wrote none.
	LastAnswer(id string) string
	// BackgroundWork is whether session id's agent has work of its own
	// running in the background, where its protocol says so.
	BackgroundWork(id string) bool
	// Relaunch closes session id's chat, runs move, and opens the chat again
	// on its conversation and settings, wherever move left them. Refused
	// while a turn runs.
	Relaunch(ctx context.Context, id string, move func() error) error
}

// SetChat wires the chat service the relay's prompts reach a chat through.
func (s *Service) SetChat(c ChatSink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chat = c
}

// Chat is whether session id is drawn as a chat (relay.Terminal).
func (s *Service) Chat(id string) bool { return s.chatSink(id) != nil }

// ChatPrompt hands text to session id's chat as its next message
// (relay.Terminal): relay.ErrNoChat for a session not drawn as one.
func (s *Service) ChatPrompt(id, text string) (relay.HandedPrompt, error) {
	sink := s.chatSink(id)
	if sink == nil {
		return relay.HandedPrompt{}, relay.ErrNoChat
	}
	return sink.Take(id, text)
}

// ChatOpened is whether session id's chat has opened its conversation
// (relay.Terminal), before which a message would land mid-handshake. A chat
// nobody opened yet is started here: the relay asks only when a message waits
// for that session, and its next poll finds the conversation open.
func (s *Service) ChatOpened(id string) bool {
	sink := s.chatSink(id)
	if sink == nil {
		return false
	}
	if sink.Opened(id) {
		return true
	}
	s.wakeChat(sink, id)
	return false
}

// wakeChat starts session id's chat in the background, once however many
// polls ask while it starts.
func (s *Service) wakeChat(sink ChatSink, id string) {
	if _, waking := s.wakingChats.LoadOrStore(id, true); waking {
		return
	}
	go func() {
		defer s.wakingChats.Delete(id)
		if err := sink.Open(context.Background(), id); err != nil {
			slog.Warn("start a chat a message waits for", "session", id, "err", err)
		}
	}()
}

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

// BackgroundWork is whether session id's agent ended its last turn with work
// still running in the background (relay.Terminal): its chat's driver says so,
// and in a terminal its last `done` report did.
func (s *Service) BackgroundWork(id string) bool {
	if sink := s.chatSink(id); sink != nil {
		return sink.BackgroundWork(id)
	}
	running, _ := s.background.Load(id)
	return running == true
}

// chatSink is the chat service when session id runs as a chat, or is drawn as
// one with no terminal running, nil otherwise. The window starts a chat only
// when it shows it, so a chat nobody opened since launch has no process yet.
func (s *Service) chatSink(id string) ChatSink {
	if !s.spawnOf(id).chat && (s.live(id) || s.store.SessionView(id) != api.ViewChat) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.chat
}

// Reachable answers whether session id has a process a message can reach:
// its terminal, or its chat, started on the first message when it does not run
// yet. Live stays the terminal's question, since a write that needs the PTY
// must not be told a chat is there to take it.
func (s *Service) Reachable(_ context.Context, opts api.SessionIDOptions) (bool, error) {
	return s.live(opts.ID) || s.Chat(opts.ID), nil
}
