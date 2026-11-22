package chat

import (
	"regexp"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/relay"
)

// unsuggested records whether the turn in flight gets no suggestion at its
// end: one lich's relay opened with an envelope, which the person did not
// write, or the person's own shell command, which no model answered.
func (c *chatSession) unsuggested(yes bool) {
	c.mu.Lock()
	c.noOffer = yes
	c.mu.Unlock()
}

// isEnvelope is whether text is a notification lich's relay wrote.
func isEnvelope(text string) bool {
	_, ok := relay.EnvelopeLine(text)
	return ok
}

// offerAfter decides, as a turn ends, whether its end is open to a
// suggestion, and remembers which turn that is: one the person opened to the
// model, ended as asked, with nothing waiting to follow it, in a session whose
// suggestions are not paused.
func (c *chatSession) offerAfter(reason string, followed bool) {
	c.mu.Lock()
	open := reason == stopEndTurn && !followed && !c.noOffer
	c.mu.Unlock()
	open = open && !c.svc.suggestionsPaused(c.id)
	c.mu.Lock()
	c.offer = 0
	if open {
		c.offer = c.turns
	}
	c.mu.Unlock()
}

// offered is the turn whose end is open to a suggestion, 0 for none.
func (c *chatSession) offered() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offer
}

// suggest shows text in the box as the person's likely next message, unless
// another turn began since the turn it was made for ended.
func (c *chatSession) suggest(turn int, text string) {
	c.mu.Lock()
	ok := text != "" && turn != 0 && c.offer == turn && !c.running
	c.mu.Unlock()
	if ok {
		events.Send(c.svc.hub.Emit, api.ChatSuggestionEvent, api.ChatSuggestion{SessionID: c.id, Text: text})
	}
}

// suggestionsPaused is whether session id makes no suggestions: a worker,
// opened by another session, until the person writes in it. The person having
// written outlives the chat's process, not lich.
func (s *Service) suggestionsPaused(id string) bool {
	s.mu.Lock()
	typed := s.typed[id]
	s.mu.Unlock()
	return !typed && s.store.SessionWorker(id)
}

// typedIn records that the person wrote in c, which ends a worker's pause.
func (s *Service) typedIn(c *chatSession) {
	s.mu.Lock()
	was := s.typed[c.id]
	s.typed[c.id] = true
	s.mu.Unlock()
	if d, ok := c.drv.(*claudeDriver); ok && !was && s.store.SessionWorker(c.id) {
		d.pauseSuggestions(false)
	}
}

// suggestionLabel is a label a model puts before its answer.
var suggestionLabel = regexp.MustCompile(`(?i)^(suggestion|response|output|answer|result)\s*:\s*`)

// cleanSuggestion is a model's reply to the suggestion prompt as the box
// shows it, the way Claude Code reads its own: trimmed, out of a wrapping tag
// and past a leading label, "" when nothing is left.
func cleanSuggestion(reply string) string {
	text := strings.TrimSpace(reply)
	for _, tag := range []string{"suggestion", "response", "output", "answer", "result"} {
		open, end := "<"+tag+">", "</"+tag+">"
		if strings.HasPrefix(strings.ToLower(text), open) && strings.HasSuffix(strings.ToLower(text), end) {
			text = strings.TrimSpace(text[len(open) : len(text)-len(end)])
			break
		}
	}
	return strings.TrimSpace(suggestionLabel.ReplaceAllString(text, ""))
}
