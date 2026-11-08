package terminal

import (
	"strings"
	"sync"
	"time"
)

// progRecord is a report as kept, with the order it arrived in.
type progRecord struct {
	progReport
	seq uint64
}

// programStatus is one session's OSC 7501 records and what the card was last
// told from them. The zero value is ready to use.
type programStatus struct {
	mu   sync.Mutex
	recs map[string]progRecord
	seq  uint64
	// hooked is whether an agent's hooks are reporting the session's state. A
	// hook outranks a program: while one is, the records are kept and the card
	// is the hook's.
	hooked bool
	// shown is the last event published from the records, zero before the
	// first and after a hook took the card over.
	shown statusEvent
	// order is held across deciding and publishing, by the records and by a
	// hook's report, so what the window hears is in the order it was decided.
	order sync.Mutex
	// dirty and running are the single publisher's: a change to publish, and
	// whether a goroutine is on it.
	dirty, running bool
	// unreadAt is when the mark was last written, unreadWant the newest value
	// waiting out progUnreadGap, and unreadArmed whether a timer will write it.
	unreadAt    time.Time
	unreadWant  bool
	unreadArmed bool
}

// kick marks the records as changed and reports whether the caller is the one
// to start the publisher.
func (p *programStatus) kick() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dirty = true
	if p.running {
		return false
	}
	p.running = true
	return true
}

// take reports whether there is a change to publish, and retires the
// publisher when there is none.
func (p *programStatus) take() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.dirty {
		p.running = false
		return false
	}
	p.dirty = false
	return true
}

// unreadDue answers whether the mark is written now, and how long until a
// held value should be, zero when no timer is owed.
func (p *programStatus) unreadDue(want bool, now time.Time) (write bool, wait time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	since := now.Sub(p.unreadAt)
	if since >= progUnreadGap {
		p.unreadAt = now
		return true, 0
	}
	p.unreadWant = want
	if p.unreadArmed {
		return false, 0
	}
	p.unreadArmed = true
	return false, progUnreadGap - since
}

// takeUnread is the held value's time coming round.
func (p *programStatus) takeUnread(now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unreadArmed = false
	p.unreadAt = now
	return p.unreadWant
}

// progReplies is the query answers pending for one session, one per terminator
// because answering twice says nothing more. The zero value is ready to use.
type progReplies struct {
	mu               sync.Mutex
	st, bel, running bool
}

// ask files one query and reports whether the caller is the one to start the
// writer.
func (r *progReplies) ask(bel bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if bel {
		r.bel = true
	} else {
		r.st = true
	}
	if r.running {
		return false
	}
	r.running = true
	return true
}

// next is the next answer to write, and retires the writer when there is none.
func (r *progReplies) next() (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.st:
		r.st = false
		return progQueryReply + "\x1b\\", true
	case r.bel:
		r.bel = false
		return progQueryReply + "\a", true
	}
	r.running = false
	return "", false
}

// apply stores one report: it replaces its record whole, and a clear removes
// the record and every record under it, or every record when it has no id.
func (p *programStatus) apply(r progReport) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if r.state == progClear {
		p.dropLocked(func(rec progRecord) bool {
			return r.id == "" || rec.id == r.id || strings.HasPrefix(rec.id, r.id+"/")
		})
		return
	}
	if p.recs == nil {
		p.recs = make(map[string]progRecord)
	}
	if _, ok := p.recs[r.id]; !ok && len(p.recs) >= progMaxRecords {
		p.evictLocked()
	}
	p.seq++
	p.recs[r.id] = progRecord{progReport: r, seq: p.seq}
}

// promptStarted is the shell starting a prompt: the program that reported
// working, blocked or idle has returned to it, and so has an agent CLI whose
// hooks held the card, most of which never report the session ended. done and
// error stay for the person to read.
func (p *programStatus) promptStarted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooked = false
	return p.dropLocked(func(rec progRecord) bool {
		return rec.state == progWorking || rec.state == progBlocked || rec.state == progIdle
	})
}

// typed is the person typing in the session: they have seen the finished
// records.
func (p *programStatus) typed() bool {
	return p.drop(func(rec progRecord) bool { return rec.state == progDone || rec.state == progError })
}

// reset is a full reset (RIS): every record goes.
func (p *programStatus) reset() bool {
	return p.drop(func(progRecord) bool { return true })
}

func (p *programStatus) drop(match func(progRecord) bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dropLocked(match)
}

func (p *programStatus) dropLocked(match func(progRecord) bool) bool {
	changed := false
	for k, rec := range p.recs {
		if match(rec) {
			delete(p.recs, k)
			changed = true
		}
	}
	return changed
}

// evictLocked removes the record updated least recently.
func (p *programStatus) evictLocked() {
	oldest, found := progRecord{}, false
	for _, rec := range p.recs {
		if !found || rec.seq < oldest.seq {
			oldest, found = rec, true
		}
	}
	delete(p.recs, oldest.id)
}

// setHooked records whether a hook speaks for the session. Taking the card
// over forgets what the records last published, so the records speak again
// with their next change once the hook has gone.
func (p *programStatus) setHooked(hooked bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hooked = hooked
	if hooked {
		p.shown = statusEvent{}
	}
}

// next is what the card should be told now, if anything. With no records
// left, a card the records drove is cleared.
func (p *programStatus) next(id string) (statusEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.hooked {
		return statusEvent{}, false
	}
	var ev statusEvent
	if best, ok := p.summaryLocked(); ok {
		ev = best.event(id)
	} else if p.shown.State != "" {
		ev = statusEvent{ID: id, State: statusIdle}
	}
	if ev.State == "" || ev == p.shown {
		return statusEvent{}, false
	}
	p.shown = ev
	return ev, true
}

// progUrgency orders states for the one record a card shows. working comes
// before done and error, which outlive a prompt: one part finished while
// another still runs is a session still running, and the card reads an error
// as a finished turn.
var progUrgency = map[string]int{progBlocked: 5, progWorking: 4, progError: 3, progDone: 2, progIdle: 1}

// summaryLocked picks the record the card shows: the most urgent state, then
// the root record, then the one updated last. A record without an app takes
// its nearest ancestor's.
func (p *programStatus) summaryLocked() (progRecord, bool) {
	var best progRecord
	found := false
	for _, rec := range p.recs {
		if !found || progOutranks(rec, best) {
			best, found = rec, true
		}
	}
	for id := best.id; found && best.app == "" && id != ""; {
		id = id[:max(strings.LastIndexByte(id, '/'), 0)]
		best.app = p.recs[id].app
	}
	return best, found
}

func progOutranks(a, b progRecord) bool {
	ua, ub := progUrgency[a.state], progUrgency[b.state]
	switch {
	case ua != ub:
		return ua > ub
	case b.id == "":
		return false
	case a.id == "":
		return true
	}
	return a.seq > b.seq
}

// event is the record in the words of the session-status contract
// (docs/hooks/session-state.md), whose fields it is capped the same way as.
// The card has no error state: an error is a finished turn, which is the mark
// that asks the person to look.
func (r progReport) event(id string) statusEvent {
	label := r.app
	if label == "" {
		label = r.title
	}
	switch r.state {
	case progWorking:
		tool, detail := label, r.msg
		if tool == "" {
			tool, detail = r.msg, ""
		}
		return statusEvent{ID: id, State: statusBusy, Tool: clampRunes(tool, hookTextLimit), Detail: clampRunes(detail, hookTextLimit)}
	case progBlocked:
		reason := r.msg
		if reason == "" {
			reason = r.title
		}
		return statusEvent{ID: id, State: statusWaiting, Reason: clampRunes(reason, hookTextLimit)}
	case progDone, progError:
		return statusEvent{ID: id, State: statusDone}
	}
	return statusEvent{ID: id, State: statusIdle}
}
