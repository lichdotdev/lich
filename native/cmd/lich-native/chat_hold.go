package main

import (
	"slices"
	"strconv"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// A message sent from the composer waits chatSendHold in the window before it
// goes to the agent, as Claude Code's prompt does, Esc or Stop taking it back
// to the box meanwhile. It is drawn sent, its turn running, so the wait does
// not show.

const chatSendHold = 2 * time.Second

// chatHeld is a message waiting to go: it, its item as the conversation draws
// it, and whether it went, which takes it out of Esc's reach.
type chatHeld struct {
	msg  chatMessage
	item backend.ChatItem
	sent bool
}

// hold keeps m as session id's message waiting to go. Its item takes the
// id and turn the backend gives the person's message (conversation.userMessage)
// so the one heard back replaces it in place, and its time is the wait's end,
// where the turn's clock starts counting.
func (s *chatStore) hold(id string, m chatMessage, now time.Time) *chatHeld {
	s.mu.Lock()
	defer s.mu.Unlock()
	turn := 0
	for _, it := range s.items[id] {
		turn = max(turn, it.Turn)
	}
	turn++
	h := &chatHeld{msg: m, item: backend.ChatItem{ID: "user-" + strconv.Itoa(turn), Kind: api.ItemUser, Text: m.text, Files: filePaths(m.files), Turn: turn, At: now.Add(chatSendHold).UnixMilli()}}
	if s.held == nil {
		s.held = map[string]*chatHeld{}
	}
	s.held[id] = h
	return h
}

// holding is whether session id has a message waiting to go.
func (s *chatStore) holding(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.held[id]
	return h != nil && !h.sent
}

// unhold takes session id's waiting message back, false once it went.
func (s *chatStore) unhold(id string) (chatMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.held[id]
	if h == nil || h.sent {
		return chatMessage{}, false
	}
	delete(s.held, id)
	return h.msg, true
}

// dispatch marks h gone to the agent, false when it was taken back first.
func (s *chatStore) dispatch(id string, h *chatHeld) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[id] != h || h.sent {
		return false
	}
	h.sent = true
	return true
}

// dropHeld forgets h, which the backend refused.
func (s *chatStore) dropHeld(id string, h *chatHeld) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held[id] == h {
		delete(s.held, id)
	}
}

// withHeld is list with session id's message waiting or gone and not yet heard
// back. s.mu is held.
func (s *chatStore) withHeld(id string, list []backend.ChatItem) []backend.ChatItem {
	h := s.held[id]
	if h == nil || slices.ContainsFunc(list, func(it backend.ChatItem) bool { return it.ID == h.item.ID }) {
		return list
	}
	return append(list, h.item)
}

// heard forgets session id's message gone once the backend's copy of a
// person's message arrives, whatever id it carries. s.mu is held.
func (s *chatStore) heard(id string, it backend.ChatItem) {
	if it.Kind == api.ItemUser {
		s.forgetSent(id)
	}
}

// forgetSent forgets session id's message gone, which the conversation now
// holds or never will: a history read again or a turn ended stand for it too,
// so a copy never heard does not keep the turn running. s.mu is held.
func (s *chatStore) forgetSent(id string) {
	if h := s.held[id]; h != nil && h.sent {
		delete(s.held, id)
	}
}

// holdChat keeps m as session id's message, sending it once the wait is up
// unless it was taken back; a refused one goes back into the composer.
func (u *window) holdChat(p *chatPane, id string, m chatMessage) {
	h := u.model.chat.hold(id, m, time.Now())
	u.model.invalidate()
	time.AfterFunc(chatSendHold, func() {
		if !u.model.chat.dispatch(id, h) {
			return
		}
		if err := u.client.ChatPrompt(u.ctx, id, m.text, m.files); err != nil {
			u.model.chat.dropHeld(id, h)
			u.chatRefused(p, m, err)
		}
	})
}

// chatRefused hands m, which the backend refused, back to the composer.
func (u *window) chatRefused(p *chatPane, m chatMessage, err error) {
	p.mu.Lock()
	p.unsent = m
	p.mu.Unlock()
	u.chatFailed(err)
}

// unholdChat takes session id's waiting message back into the box, its text
// ahead of what was typed since and its cards ahead of those added, and
// reports whether there was one.
func (u *window) unholdChat(p *chatPane, id string) bool {
	m, ok := u.model.chat.unhold(id)
	if !ok {
		return false
	}
	p.restore(m)
	// This frame read the conversation with the message in it.
	u.model.invalidate()
	return true
}
