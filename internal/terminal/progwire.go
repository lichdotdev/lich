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
	// progUnreadGap is how often a program's reports may write the session's
	// unread mark to the database.
	progUnreadGap = 30 * time.Second
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

// programReport applies one OSC 7501 payload and reports whether it was a
// report, which may have changed the records.
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

// kickProgramStatus asks for the records to be published. A service's PTY has
// no card.
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

// noteProgramUnread writes a program's unread mark at most once per
// progUnreadGap, the newest value winning at the end of the gap.
func (s *Service) noteProgramUnread(id string, sess *session, unread bool) {
	write, wait := sess.prog.unreadDue(unread, time.Now())
	if wait > 0 {
		time.AfterFunc(wait, func() {
			latest := sess.prog.takeUnread(time.Now())
			if s.sessionOf(id) == sess {
				s.noteUnread(id, latest)
			}
		})
	}
	if write {
		s.noteUnread(id, unread)
	}
}

// programTyped is the person typing in a session, which ends its finished
// records.
func (s *Service) programTyped(id string) {
	if sess := s.sessionOf(id); sess != nil && sess.prog.typed() {
		s.kickProgramStatus(id, sess)
	}
}

// publishHookStatus publishes a report a hook made and the window is to hear.
// From here until the hook reports the session ended or the shell starts a
// prompt, the card is the hook's.
func (s *Service) publishHookStatus(ev statusEvent) {
	sess := s.sessionOf(ev.ID)
	if sess == nil {
		s.publishStatus(ev)
		return
	}
	sess.prog.order.Lock()
	defer sess.prog.order.Unlock()
	sess.prog.setHooked(ev.State != statusIdle)
	s.publishStatus(ev)
}

func (s *Service) sessionOf(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}
