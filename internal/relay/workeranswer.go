package relay

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// A worker its caller closed: the way a subagent errand ends that a native
// subagent has in TaskStop.

// taskMessage composes a ticket's task envelope for the target it is handed to
// now. An errand at a worker whose turn's end lich sees and whose final message
// it reads is answered by that message (answerByTurn); the rest answer by
// their ticket.
func (s *Service) taskMessage(id string, t *ticket, kind string) string {
	answer := AnswerReply
	said, readable := s.term.FinalMessage(t.targetID)
	if readable && (s.term.Chat(t.targetID) || s.reportsTurnEnd(kind)) {
		s.mu.Lock()
		t.turnAnswers, t.saidBefore, answer = true, said, AnswerLastMessage
		s.mu.Unlock()
	}
	message := taskEnvelope(s.lang(), id, t, answer, s.offersTools(kind)).String()
	if t.loop {
		message += "\n\n" + loopEnvelope(s.lang(), t).String()
	}
	return message
}

// lapseUnanswered ends errand id as unanswered, the way a turn ending without
// a reply ends one, unless it ended meanwhile: it stays answerable by its
// ticket (lapseLocked), and an errand nobody is attending goes to the sender's
// inbox.
func (s *Service) lapseUnanswered(id string, t *ticket) {
	s.mu.Lock()
	if s.tickets[id] != t {
		s.mu.Unlock()
		return
	}
	delete(s.tickets, id)
	close(t.stalled)
	s.lapseLocked(id, t, StatusUnanswered)
	unattended := t.attended == 0
	if unattended {
		s.stashLocked(id, t, StatusUnanswered, "")
	}
	s.mu.Unlock()

	s.clear(t)
	if s.events != nil {
		events.Send(s.events.Emit, api.RelayStalledEvent, StalledEvent{ID: t.fromID, TargetID: t.targetID, Target: t.target})
	}
	if unattended {
		s.announceInbox(t.fromID)
	}
}

// owesSubagentAnswerLocked is whether a subagent errand still waits on
// sessionID's answer: someone opened it as their subagent and has not heard
// back. Called under s.mu.
func (s *Service) owesSubagentAnswerLocked(sessionID string) bool {
	for _, t := range s.tickets {
		if t.targetID == sessionID && t.subagent {
			return true
		}
	}
	for _, t := range s.lapsed {
		if t.targetID == sessionID && t.subagent {
			return true
		}
	}
	return false
}

// awaitsOutcomeLocked is whether an outcome is still on its way to sessionID's
// prompt: an errand it sent is open, or a result for it has not been read by a
// turn of its own yet (inboxEntry.seen). A private errand never reaches that
// prompt, so it is not counted: nothing would ever resume the session for it.
// Called under s.mu.
func (s *Service) awaitsOutcomeLocked(sessionID string) bool {
	for _, t := range s.tickets {
		if t.fromID == sessionID && !t.private {
			return true
		}
	}
	for _, e := range s.ready {
		if e.fromID == sessionID && !e.seen {
			return true
		}
	}
	for _, e := range s.held {
		if e.fromID == sessionID && !e.private && !e.seen {
			return true
		}
	}
	return false
}

// seeNewsLocked marks every result sessionID was already told about as read: a
// turn starting after the notice is the one the notice started, or one that
// came after it. Called under s.mu on a busy report.
func (s *Service) seeNewsLocked(sessionID string) {
	for _, e := range s.ready {
		if e.fromID == sessionID && e.nudged {
			e.seen = true
		}
	}
	for _, e := range s.held {
		if e.fromID == sessionID && e.nudged {
			e.seen = true
		}
	}
}

// finishIfTurnEnded finishes a worker whose answer landed after the turn it
// answered in had already ended: the final message is read after the done
// report. One landing mid-turn is finished by that done
// (finishedWorkerLocked).
func (s *Service) finishIfTurnEnded(workerID string) {
	s.mu.Lock()
	finish := s.reportedWorkers[workerID] && s.state[workerID] != stateBusy &&
		!s.errandOpenAtLocked(workerID) && !s.awaitsOutcomeLocked(workerID) && s.workerFinished != nil
	if finish {
		delete(s.reportedWorkers, workerID)
	}
	s.mu.Unlock()
	if finish {
		go s.finishWorker(workerID)
	}
}

// errandOpenAtLocked is whether any errand is still open at sessionID. Called
// under s.mu.
func (s *Service) errandOpenAtLocked(sessionID string) bool {
	for _, t := range s.tickets {
		if t.targetID == sessionID {
			return true
		}
	}
	return false
}

// SessionClosed ends the subagent errands at a session that was just closed by
// closerID, empty when the window closed it. A caller that closed its own
// worker (TaskStop runs `lich close` in the caller's process) stopped the work
// and is told nothing; a caller still holding the line hears stopped, and the
// ticket stays waitable as stopped. Anyone else's close ends work the caller
// never stopped, so the stop is filed in the caller's inbox with a nudge, as is
// one for a caller that is itself a worker somebody waits on
// (owesSubagentAnswerLocked), whose answer waits for this outcome. The terminal calls this before the process dies
// (terminal.SetSessionClosed), so the SessionEnd its CLI reports on the way out
// finds nothing to call unanswered. Errands of any other kind end as they
// always did. The session's reported state goes too: the same id reopened is
// a new process that has reported nothing yet.
func (s *Service) SessionClosed(sessionID, closerID string) {
	var stopped []*ticket
	var told []string
	by := ""
	if closerID != "" {
		by = s.labelOf(closerID)
	}
	s.mu.Lock()
	delete(s.reportedWorkers, sessionID)
	s.forgetStateLocked(sessionID)
	for id, t := range s.tickets {
		if t.targetID != sessionID || !t.subagent {
			continue
		}
		delete(s.tickets, id)
		t.stoppedBy = by
		if t.attended == 0 && (t.fromID != closerID || s.owesSubagentAnswerLocked(t.fromID)) {
			s.stashLocked(id, t, StatusStopped, "")
			told = append(told, t.fromID)
		} else {
			s.lapseLocked(id, t, StatusStopped)
		}
		t.stopped = true
		close(t.stalled)
		stopped = append(stopped, t)
	}
	s.mu.Unlock()
	s.clearAll(stopped)
	s.announceInboxAll(told)
}

// answerByTurn answers errand e, which workerID's turn that just ended was,
// with that turn's final message (docs/hooks/notification.md, Which errands
// use it), by the path Reply takes: an answer that came first by the ticket
// wins, and this one is dropped. A turn that wrote nothing new, read
// back as the message on record when the task went in, ends the errand
// unanswered as a turn without a reply always has.
func (s *Service) answerByTurn(workerID string, e endedErrand) {
	text := s.newFinalMessage(workerID, e.t.saidBefore)
	if text == "" {
		s.lapseUnanswered(e.id, e.t)
		return
	}
	if _, err := s.Reply(context.Background(), ReplyOptions{From: workerID, Ticket: e.id, Answer: text}); err != nil {
		slog.Debug("relay: a turn's final message was not taken", "session", workerID, "err", err)
		return
	}
	s.finishIfTurnEnded(workerID)
}

// defaultFinalSettle is how long a turn's end rereads a worker's final message
// before it takes the turn as one that wrote nothing: a provider's done hook
// can run before its transcript holds the last words.
const defaultFinalSettle = 1500 * time.Millisecond

// finalPoll is how often that wait rereads.
const finalPoll = 100 * time.Millisecond

// newFinalMessage is workerID's final message once it differs from before,
// "" when it is blank or never moved off before within finalSettle.
func (s *Service) newFinalMessage(workerID, before string) string {
	deadline := s.now().Add(s.finalSettle)
	for {
		text, _ := s.term.FinalMessage(workerID)
		if strings.TrimSpace(text) != "" && text != before {
			return text
		}
		if !s.now().Before(deadline) {
			return ""
		}
		time.Sleep(finalPoll)
	}
}
