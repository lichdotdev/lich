package main

import "github.com/lichdotdev/lich/api"

// suggest takes what the backend predicts session s.SessionID's person types
// next; an empty text clears it.
func (s *chatStore) suggest(u api.ChatSuggestion) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.suggestion == nil {
		s.suggestion = map[string]string{}
	}
	s.suggestion[u.SessionID] = u.Text
}

// dropSuggestion forgets session id's suggestion once the person took it.
// Typing only hides it: a box emptied again shows it back, until a message
// goes (hold, enqueue).
func (s *chatStore) dropSuggestion(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.suggestion, id)
}

func (s *chatStore) suggestionOf(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.suggestion[id]
}

// shownSuggestion is the suggestion session id's box draws in place of its
// placeholder, "" when there is none or the box is not idle: it holds text, a
// file or a quote, a turn runs or a message waits to go, or the queue holds one.
func (u *window) shownSuggestion(p *chatPane, id string) string {
	if p.editor.Len() > 0 || len(p.attached) > 0 || len(p.quotes) > 0 || p.tools.opened() || p.picker.panel.Opened() {
		return ""
	}
	if u.model.chat.isRunning(id) || len(u.model.chat.queued(id)) > 0 {
		return ""
	}
	return u.model.chat.suggestionOf(id)
}

// takeSuggestion is Tab on the box showing one: the suggestion becomes the
// box's text, to edit or send with Enter.
func (u *window) takeSuggestion(p *chatPane, id string) {
	text := u.shownSuggestion(p, id)
	if text == "" {
		return
	}
	p.editor.SetText(text)
	n := p.editor.Len()
	p.editor.SetCaret(n, n)
	u.model.chat.dropSuggestion(id)
}
