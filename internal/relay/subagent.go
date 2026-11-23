package relay

import (
	"log/slog"
	"slices"

	"github.com/lichdotdev/lich/internal/prompt"
)

// A subagent errand (SendSubagent) is an ordinary one with four differences,
// all here: its report reaches the sender whole rather than as a nudge to
// collect it, the sender hears when its worker blocks on a permission, sweep
// leaves it alone while the worker runs (workerHolds), and the worker is
// finished once it reported and its turn ended (finishedWorkerLocked).

// tellNews delivers one flush's news to fromID, one envelope per outcome in
// one message. Where the sender's chat takes it, a result goes in whole: the
// model reads it the way it reads a native subagent's result, without a
// collect call. A sender in its terminal is typed the same envelopes with each
// result left out, and the results go back to the inbox for the collect they
// name: a long report typed into a TUI is pasted text the person has to wade
// through.
func (s *Service) tellNews(fromID string, n news, hasTools bool) error {
	lang := s.lang()
	handled, err := s.handToChat(fromID, newsMessage(lang, n.entries, n.results, hasTools))
	if err != nil {
		return err
	}
	if handled {
		s.announceInbox(fromID)
		return nil
	}
	s.mu.Lock()
	s.restoreReportsLocked(n.results)
	s.mu.Unlock()
	return s.typeIn(fromID, newsMessage(lang, n.entries, nil, hasTools))
}

// newsMessage is one envelope per entry, in order, with the result in for each
// entry of whole.
func newsMessage(lang prompt.Lang, entries, whole []*inboxEntry, hasTools bool) string {
	envs := make([]Envelope, len(entries))
	for i, e := range entries {
		envs[i] = outcomeEnvelope(lang, e, slices.Contains(whole, e), hasTools)
	}
	return joinEnvelopes(envs)
}

// restoreReportsLocked puts results takeNewsLocked moved to held back in the
// inbox, unless a Wait on their ticket took them meanwhile. Called under s.mu.
func (s *Service) restoreReportsLocked(reports []*inboxEntry) {
	for _, e := range reports {
		if s.held[e.ticket] == e {
			delete(s.held, e.ticket)
			s.ready[e.ticket] = e
		}
	}
}

// blockedSendersLocked marks every subagent errand delivered to sessionID as
// told about the block it has just reported, and returns the senders not told
// yet. A busy or done report ends the block (unblockLocked). Called under s.mu,
// before recordState, while s.state still says whether a turn is running: a
// waiting outside one is a prompt at rest, not a block.
func (s *Service) blockedSendersLocked(sessionID string) []endedErrand {
	if s.state[sessionID] != stateBusy {
		return nil
	}
	var told []endedErrand
	for id, t := range s.tickets {
		if t.targetID != sessionID || !t.subagent || !t.submitted || t.blockNoted {
			continue
		}
		t.blockNoted = true
		told = append(told, endedErrand{id, t})
	}
	return told
}

// unblockLocked ends the block sessionID was in for its subagent errands, so
// the next one is told again. Called under s.mu.
func (s *Service) unblockLocked(sessionID string) {
	for _, t := range s.tickets {
		if t.targetID == sessionID {
			t.blockNoted = false
		}
	}
}

// SetWorkerFinished wires what runs for a worker that finished: it answered its
// subagent errand and the turn it answered in ended, with no other errand open
// at it. The app closes the worker there when it shares its caller's checkout
// (spawn.CloseFinishedWorker). Called at startup, before any errand exists.
func (s *Service) SetWorkerFinished(fn func(workerID string) error) {
	s.workerFinished = fn
}

// noteWorkerReportedLocked marks the target of a subagent errand that was just
// answered, for the end of its turn to finish. Called under s.mu.
func (s *Service) noteWorkerReportedLocked(t *ticket) {
	if t.subagent {
		s.reportedWorkers[t.targetID] = true
	}
}

// finishedWorkerLocked reads one state report for a worker that reported this
// turn: a done finishes it unless another errand is still open at it, or one
// it sent has not come back (a worker that answered by its ticket while its own
// worker runs), checked before endedErrands takes this turn's errands off the
// table, since one of those may still be answered late. An interrupt or the session ending clears
// the mark without finishing it: whoever stopped that turn is still at its
// card. Called under s.mu.
func (s *Service) finishedWorkerLocked(sessionID, state string) bool {
	if !s.reportedWorkers[sessionID] {
		return false
	}
	switch state {
	case stateDone:
	case stateInterrupted, stateIdle:
		delete(s.reportedWorkers, sessionID)
		return false
	default:
		return false
	}
	delete(s.reportedWorkers, sessionID)
	return !s.errandOpenAtLocked(sessionID) && !s.awaitsOutcomeLocked(sessionID) && s.workerFinished != nil
}

func (s *Service) finishWorker(workerID string) {
	if err := s.workerFinished(workerID); err != nil {
		slog.Warn("relay: close a finished worker", "session", workerID, "err", err)
	}
}
