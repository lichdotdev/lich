//go:build !windows

package chat

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// New starts the chat again on an empty conversation: what the old one said
// is gone from the chat.
func TestNewStartsAnEmptyConversation(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	r.prompt("hi")
	r.eventually("the answer", func() bool { _, ok := r.item(api.ItemAgent, "Oi!"); return ok })
	if _, err := r.svc.New(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("New: %v", err)
	}
	if h := r.history(); len(h) != 0 {
		t.Errorf("history after New: %v, want none", h)
	}
}
