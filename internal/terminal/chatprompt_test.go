//go:build !windows

package terminal

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/relay"
)

// wakeSink is a chat service whose chats open when Open is called.
type wakeSink struct {
	mu     sync.Mutex
	opens  map[string]int
	opened map[string]bool
	gate   chan struct{}
}

func (w *wakeSink) Open(_ context.Context, id string) error {
	<-w.gate
	w.mu.Lock()
	defer w.mu.Unlock()
	w.opens[id]++
	w.opened[id] = true
	return nil
}

func (w *wakeSink) Opened(id string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.opened[id]
}

func (w *wakeSink) openCount(id string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.opens[id]
}

func (*wakeSink) Take(string, string) (relay.HandedPrompt, error) { return relay.HandedPrompt{}, nil }
func (*wakeSink) LastAnswer(string) string                        { return "" }
func (*wakeSink) BackgroundWork(string) bool                      { return false }
func (*wakeSink) Relaunch(context.Context, string, func() error) error {
	return nil
}

// TestAChatNobodyOpenedStartsOnItsFirstMessage pins the relay's reach into a
// chat the window has not shown since launch: it is in the roster, and the
// delivery's polls start it once, then find it open.
func TestAChatNobodyOpenedStartsOnItsFirstMessage(t *testing.T) {
	t.Parallel()
	svc := New(stubBins{views: map[string]string{"chat": api.ViewChat}}, nil, events.New())
	sink := &wakeSink{opens: map[string]int{}, opened: map[string]bool{}, gate: make(chan struct{})}
	svc.SetChat(sink)

	for _, id := range []string{"chat", "term"} {
		want := id == "chat"
		if got, _ := svc.Reachable(context.Background(), api.SessionIDOptions{ID: id}); got != want {
			t.Errorf("Reachable(%s) = %v, want %v", id, got, want)
		}
		if got := svc.Chat(id); got != want {
			t.Errorf("Chat(%s) = %v, want %v", id, got, want)
		}
	}

	for range 3 {
		if svc.ChatOpened("chat") {
			t.Fatal("ChatOpened before the chat started")
		}
	}
	close(sink.gate)
	deadline := time.Now().Add(5 * time.Second)
	for !svc.ChatOpened("chat") {
		if time.Now().After(deadline) {
			t.Fatal("the chat never opened")
		}
		time.Sleep(time.Millisecond)
	}
	if n := sink.openCount("chat"); n != 1 {
		t.Errorf("Open ran %d times, want 1", n)
	}
	if svc.ChatOpened("term") || sink.openCount("term") != 0 {
		t.Error("a session drawn as its terminal was started as a chat")
	}
}
