package chat

import (
	"fmt"
	"log/slog"

	"github.com/lichdotdev/lich/api"
)

// question is a question item the agent waits on.
type question struct {
	asks  []api.ChatAsk
	reply func(api.ChatQReplied) error
}

// question shows the agent's questions and waits for the person. Nothing
// answers them on the person's behalf, not even a checkout that answers every
// permission yes: they ask for what only the person knows.
func (c *chatSession) question(itemID string, q api.ChatQuestion, reply func(api.ChatQReplied) error) {
	c.mu.Lock()
	c.questions[itemID] = question{asks: q.Questions, reply: reply}
	c.mu.Unlock()
	c.conv.update(itemID, api.ItemQuestion, true, func(it *api.ChatItem) {
		it.Kind, it.Tool, it.Question = api.ItemQuestion, nil, &q
	})
	detail := ""
	if len(q.Questions) > 0 {
		detail = q.Questions[0].Text
	}
	c.svc.term.ReportChatState(c.id, stateWaiting, "", detail)
}

// reply answers question item itemID.
func (c *chatSession) reply(itemID string, r api.ChatQReplied) error {
	c.mu.Lock()
	q, ok := c.questions[itemID]
	if ok {
		delete(c.questions, itemID)
	}
	c.mu.Unlock()
	if !ok {
		return fmt.Errorf("question %q is not waiting for an answer", itemID)
	}
	if !r.Declined && len(r.Answers) != len(q.asks) {
		c.mu.Lock()
		c.questions[itemID] = q
		c.mu.Unlock()
		return fmt.Errorf("question %q asks %d questions, and %d answers came", itemID, len(q.asks), len(r.Answers))
	}
	if err := q.reply(r); err != nil {
		return err
	}
	c.replied(itemID, r)
	c.svc.term.ReportChatState(c.id, stateBusy, "", "")
	return nil
}

// replied records how question item itemID was answered, once.
func (c *chatSession) replied(itemID string, r api.ChatQReplied) {
	c.conv.update(itemID, api.ItemQuestion, true, func(it *api.ChatItem) {
		if it.Question != nil && it.Question.Reply == nil {
			it.Question.Reply = &r
		}
	})
}

// declineQuestions answers no to every question still waiting, and reports
// whether there was one.
func (c *chatSession) declineQuestions() bool {
	c.mu.Lock()
	waiting := c.questions
	c.questions = map[string]question{}
	c.mu.Unlock()
	declined := api.ChatQReplied{Declined: true}
	for id, q := range waiting {
		if err := q.reply(declined); err != nil {
			slog.Warn("chat: decline a question", "session", c.id, "err", err)
		}
		c.replied(id, declined)
	}
	if len(waiting) == 0 {
		return false
	}
	c.svc.term.ReportChatState(c.id, stateBusy, "", "")
	return true
}

// declineUnanswered marks every question of a history that never got its
// answer as declined: the process that asked it is gone.
func (c *chatSession) declineUnanswered() {
	for _, it := range c.conv.snapshot() {
		if it.Kind == api.ItemQuestion && it.Question != nil && it.Question.Reply == nil {
			c.replied(it.ID, api.ChatQReplied{Declined: true})
		}
	}
}

// withdraw closes item itemID, a permission or a question the provider
// stopped waiting on, without answering it.
func (c *chatSession) withdraw(itemID string) {
	c.mu.Lock()
	_, perm := c.perms[itemID]
	_, asked := c.questions[itemID]
	delete(c.perms, itemID)
	delete(c.questions, itemID)
	c.mu.Unlock()
	switch {
	case perm:
		c.conv.update(itemID, api.ItemPermission, true, func(it *api.ChatItem) {
			if it.Permission != nil {
				it.Permission.Tool.State = api.ToolFailed
			}
		})
	case asked:
		c.replied(itemID, api.ChatQReplied{Declined: true})
	}
}
