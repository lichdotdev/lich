package terminal

import (
	"log/slog"
	"time"
)

const (
	// progPublishGap is the least time between two publishes of one session's
	// program status. Short enough that the card reads as immediate, and it
	// is what bounds the events a program printing reports in a loop can raise.
	progPublishGap = 100 * time.Millisecond
	// progUnreadBurst is how many unread writes a program's reports may cause
	// per progUnreadWindow. A build reports two changes a run; past this, the
	// program is flipping and the mark is not what it is saying.
	progUnreadBurst  = 4
	progUnreadWindow = 30 * time.Second
)

// readProgramStatus reads one chunk of a session's output for OSC 7501. It
// runs on the PTY's read goroutine, so nothing here waits: the query's answer
// and the publish each go out on a goroutine of their own.
func (s *Service) readProgramStatus(id string, sess *session, chunk []byte) {
	changed := false
	for _, sig := range sess.progScan.scan(chunk) {
		switch {
		case sig.reset:
			changed = sess.prog.reset() || changed
		case sig.prompt:
			changed = sess.prog.promptStarted() || changed
		default:
			changed = s.programReport(id, sess, sig) || changed
		}
	}
	if changed {
		s.kickProgramStatus(id, sess)
	}
}

func (s *Service) programReport(id string, sess *session, sig progSignal) bool {
	body := sig.payload[len(progPrefix):]
	if progQuery(body) {
		if sess.progReplies.ask(sig.bel) {
			go s.writeProgramReplies(id, sess)
		}
		return false
	}
	r, ok := parseProgReport(body, progSeqLen(len(sig.payload), sig.bel))
	if ok {
		sess.prog.apply(r)
	}
	return ok
}

// writeProgramReplies answers the queries pending for a session. The backend
// answers rather than the window: the window's emulator does not (it sets no
// program status callback), and a session runs on with no window attached. A
// child that is not reading its input blocks this write, and only this one.
func (s *Service) writeProgramReplies(id string, sess *session) {
	for {
		reply, ok := sess.progReplies.next()
		if !ok {
			return
		}
		// A respawn under this id is a program that never asked.
		if s.sessionOf(id) != sess {
			continue
		}
		if err := s.writeBytes(id, []byte(reply)); err != nil {
			slog.Debug("terminal: program status reply", "session", id, "err", err)
		}
	}
}

// A service's PTY has no card.
func (s *Service) kickProgramStatus(id string, sess *session) {
	if sess.service == nil && sess.prog.kick() {
		go s.publishProgramStatus(id, sess)
	}
}

// publishProgramStatus publishes the session's records until they stop
// changing. Reports that arrive during a publish fold into the next, so the
// card goes to the newest state however often a program changes it.
func (s *Service) publishProgramStatus(id string, sess *session) {
	for sess.prog.take() {
		if s.sessionOf(id) != sess {
			continue
		}
		ev, published := s.publishProgramOnce(id, sess)
		if !published {
			continue
		}
		s.noteProgramUnread(id, sess, ev.State == statusDone)
		time.Sleep(progPublishGap)
	}
}

// publishProgramOnce decides and publishes under the session's order, which a
// hook's report takes too: a decision made before a hook took the card must
// not land after the hook's own report.
func (s *Service) publishProgramOnce(id string, sess *session) (statusEvent, bool) {
	sess.prog.order.Lock()
	defer sess.prog.order.Unlock()
	ev, ok := sess.prog.next(id)
	if !ok {
		return statusEvent{}, false
	}
	ev.Program = true
	s.publishStatus(ev)
	return ev, true
}

// noteProgramUnread writes the mark when it changes or not at all, never later:
// a value held back would undo a card read in between it and its write.
func (s *Service) noteProgramUnread(id string, sess *session, unread bool) {
	if sess.prog.unreadAllowed(unread, time.Now()) {
		s.noteUnread(id, unread)
	}
}

func (s *Service) programTyped(id string) {
	if sess := s.sessionOf(id); sess != nil && sess.prog.typed() {
		s.kickProgramStatus(id, sess)
	}
}

// publishHookStatus publishes a report a hook made and the window is to hear.
// From here until the hook reports the session ended or the shell starts a
// prompt, the card is the hook's; when it hands the card back, the records
// that piled up meanwhile say what the session is doing.
func (s *Service) publishHookStatus(ev statusEvent) {
	sess := s.sessionOf(ev.ID)
	if sess == nil {
		s.publishStatus(ev)
		return
	}
	sess.prog.order.Lock()
	sess.prog.setHooked(ev.State != statusIdle)
	s.publishStatus(ev)
	sess.prog.order.Unlock()
	if ev.State == statusIdle {
		s.kickProgramStatus(ev.ID, sess)
	}
}

func (s *Service) sessionOf(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}
