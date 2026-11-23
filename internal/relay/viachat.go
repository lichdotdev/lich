package relay

import (
	"context"
	"errors"
	"time"

	"github.com/lichdotdev/lich/api"
)

// ErrNoChat is what Terminal.ChatPrompt answers for a session not drawn as a
// chat, which takes its messages typed at its prompt.
var ErrNoChat = errors.New("this session is not drawn as a chat")

// How far a prompt handed to a chat got.
const (
	PromptDelivered = "delivered" // queued behind the turn running, which has not ended yet
	PromptAcked     = "acked"     // the turn carrying it started
	PromptEnded     = "ended"     // the chat exited first
)

// PromptReceipt is what became of one prompt handed to a chat.
type PromptReceipt struct {
	State string
}

// HandedPrompt is a prompt queued for a session's chat. Await waits for the
// turn carrying it to start until ctx ends, and says how far it got.
type HandedPrompt struct {
	Await func(context.Context) PromptReceipt
}

// defaultChatTurnWait bounds the wait on a chat's turn starting for a delivery
// that carries no ticket. An idle chat starts it at once; past this the chat
// is busy, and the prompt runs once its turn ends.
const defaultChatTurnWait = 10 * time.Second

// takesDelivery is whether a message can go to a session now: its prompt is
// free, or it is a chat whose conversation is open, which queues the message
// behind any turn running.
func (s *Service) takesDelivery(id string) (bool, error) {
	if s.term.Chat(id) {
		return s.term.ChatOpened(id), nil
	}
	return s.term.Ready(context.Background(), api.SessionIDOptions{ID: id})
}

// handToChat hands a message to the session's chat and reports whether it was
// the chat's to deliver: false with no error means type it instead, which is
// what a session in its terminal wants.
func (s *Service) handToChat(sessionID, message string) (bool, error) {
	handed, err := s.term.ChatPrompt(sessionID, message)
	if errors.Is(err, ErrNoChat) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.chatTurnWait)
	defer cancel()
	if handed.Await(ctx).State == PromptEnded {
		return true, errors.New("the session ended before its chat took it")
	}
	return true, nil
}
