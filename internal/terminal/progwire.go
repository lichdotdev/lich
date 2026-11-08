package terminal

import "log/slog"

// readProgramStatus reads one chunk of a session's output for OSC 7501. The
// backend answers the query, rather than the window: the window's emulator
// does not (it sets no program status callback), and a session runs on with
// no window attached.
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
		s.syncProgramStatus(id, sess)
	}
}

// programReport applies one OSC 7501 payload and reports whether it was a
// report, which may have changed the records.
func (s *Service) programReport(id string, sess *session, sig progSignal) bool {
	body := sig.payload[len(progPrefix):]
	if progQuery(body) {
		reply := progQueryReply + "\x1b\\"
		if sig.bel {
			reply = progQueryReply + "\a"
		}
		// Off the read loop: a child that is not reading its input would
		// block this write, and with it the read that drains its output.
		go func() {
			if err := s.writeBytes(id, []byte(reply)); err != nil {
				slog.Debug("terminal: program status reply", "session", id, "err", err)
			}
		}()
		return false
	}
	r, ok := parseProgReport(body, progSeqLen(len(sig.payload), sig.bel))
	if ok {
		sess.prog.apply(r)
	}
	return ok
}

// syncProgramStatus tells the card what the records now say. A service's PTY
// has no card.
func (s *Service) syncProgramStatus(id string, sess *session) {
	if sess.service != nil {
		return
	}
	ev, publish := sess.prog.next(id)
	if !publish {
		return
	}
	ev.Program = true
	s.noteUnread(id, ev.State == statusDone)
	s.publishStatus(ev)
}

// programTyped is the person typing in a session, which ends its finished
// records.
func (s *Service) programTyped(id string) {
	if sess := s.sessionOf(id); sess != nil && sess.prog.typed() {
		s.syncProgramStatus(id, sess)
	}
}

// programHooked records that a hook reported a state for the session: until
// it reports the session ended or the shell starts a prompt, the card is the
// hook's.
func (s *Service) programHooked(id, state string) {
	if sess := s.sessionOf(id); sess != nil {
		sess.prog.setHooked(state != statusIdle)
	}
}

func (s *Service) sessionOf(id string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}
