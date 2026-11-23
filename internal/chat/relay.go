package chat

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
)

// claimPoll is how often a start waits on a switch or another start of the
// same session to let go of it.
const claimPoll = 50 * time.Millisecond

// errNoChat is a message for a session whose chat is not running, or is
// still opening its conversation.
var errNoChat = errors.New("this session's chat is not running or not open yet")

// claimWhenFree is claim, waiting out a switch or start already holding id
// rather than failing: the window opening a session's chat and the spawn that
// just created it both start it at once.
func (s *Service) claimWhenFree(ctx context.Context, id string) (func(), error) {
	for {
		release, err := s.claim(id)
		if err == nil {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(claimPoll):
		}
	}
}

// Drawn is whether session id is drawn as a chat (spawn.Chat).
func (s *Service) Drawn(id string) bool { return s.store.SessionView(id) == api.ViewChat }

// Open starts session id's chat on its conversation, unless it runs already,
// and returns once the conversation is open.
func (s *Service) Open(ctx context.Context, id string) error {
	_, err := s.ensure(ctx, id)
	return err
}

// Opened is whether session id's chat runs and has opened its conversation
// (terminal.ChatSink).
func (s *Service) Opened(id string) bool {
	c := s.running(id)
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opened
}

// Take sends text as session id's next message: now when no turn runs, after
// the turns before it otherwise (terminal.ChatSink). Its receipt is acked when
// the turn carrying it starts.
func (s *Service) Take(id, text string) (relay.HandedPrompt, error) {
	c := s.running(id)
	if c == nil || !s.Opened(id) {
		return relay.HandedPrompt{}, errNoChat
	}
	c.conv.userMessage(text, nil)
	began, err := c.take(message{text: text})
	if err != nil {
		return relay.HandedPrompt{}, err
	}
	return relay.HandedPrompt{
		Await: func(ctx context.Context) relay.PromptReceipt {
			select {
			case <-began:
				return relay.PromptReceipt{State: relay.PromptAcked}
			case <-c.exited:
				return relay.PromptReceipt{State: relay.PromptEnded}
			case <-ctx.Done():
				return relay.PromptReceipt{State: relay.PromptDelivered}
			}
		},
	}, nil
}

// LastAnswer is the final message of session id's last turn, "" when that
// turn wrote none or no chat runs there (terminal.ChatSink).
func (s *Service) LastAnswer(id string) string {
	c := s.running(id)
	if c == nil {
		return ""
	}
	items := c.conv.snapshot()
	last := 0
	for _, it := range items {
		last = max(last, it.Turn)
	}
	for _, it := range slices.Backward(items) {
		if it.Turn == last && it.Kind == api.ItemAgent {
			return it.Text
		}
	}
	return ""
}

// take sends m in a turn of its own, after the turn running if one is, and
// answers a channel that closes once the turn carrying it has started. A
// question the agent waits on is left for the person: the relay's message is
// never written into the running turn (docs/hooks/notification.md, Delivery).
func (c *chatSession) take(m message) (<-chan struct{}, error) {
	started := make(chan struct{})
	close(started)
	for {
		if err := c.beginTurn(); err == nil {
			return started, c.send(m)
		}
		began := make(chan struct{})
		if c.hold(pending{msg: m, began: began}) {
			return began, nil
		}
	}
}
