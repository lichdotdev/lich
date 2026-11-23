package relay

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/store"
)

// editWindow is how long an edit stays news to another session. It is the
// window lich-plugin's edit-guard mod used (its docs/edit-guard.md): it spans
// the turn in which the other session made its edit, and past it that work is
// likely finished and the file as read is the news.
const editWindow = 10 * time.Minute

// editCLI is how a session without lich's tools reaches another one.
const editCLI = `"$LICH_BIN" send`

// edit is one session's last touch of a path.
type edit struct {
	session string
	at      time.Time
}

// overlap is a path the told session edited after another session did.
type overlap struct {
	path string
	edit
}

// edits is in memory only: a restart forgets every edit and every held note,
// so the first edit after one tells nobody.
type edits struct {
	mu sync.Mutex
	// last is the newest edit of each path. A path is absolute, so it names its
	// checkout as well.
	last map[string]edit
	// held is the overlaps waiting to be told, by the session they are told to.
	held map[string][]overlap
}

// Touched answers a /session-touched call (docs/hooks/session-touched.md): the
// edit-overlap notes for its session, "" when there is nothing to say. A held
// touch records and answers nothing; its note waits for the next answer.
func (s *Service) Touched(req api.TouchedRequest) string {
	if !req.Collect {
		s.recordEdits(req.SessionID, req.Paths)
		if req.Hold {
			return ""
		}
	}
	return s.takeEditNote(req.SessionID)
}

// recordEdits makes session the last editor of paths, holding an overlap for
// each one another session edited within editWindow.
func (s *Service) recordEdits(session string, paths []string) {
	if len(paths) == 0 {
		return
	}
	now := s.now()
	s.edits.mu.Lock()
	defer s.edits.mu.Unlock()
	s.edits.pruneLocked(now)
	for _, path := range paths {
		if prev, ok := s.edits.last[path]; ok && prev.session != session {
			s.edits.held[session] = append(s.edits.held[session], overlap{path, prev})
		}
		s.edits.last[path] = edit{session, now}
	}
}

// pruneLocked drops what left the window. Both maps stay as small as the edits
// of the last editWindow.
func (e *edits) pruneLocked(now time.Time) {
	for path, ed := range e.last {
		if now.Sub(ed.at) > editWindow {
			delete(e.last, path)
		}
	}
	for session, held := range e.held {
		held = slices.DeleteFunc(held, func(o overlap) bool { return now.Sub(o.at) > editWindow })
		if len(held) == 0 {
			delete(e.held, session)
			continue
		}
		e.held[session] = held
	}
}

// takeEditNote hands over the overlaps held for session, one envelope per other
// session, oldest first. An overlap gone stale while held, or whose other
// session no longer runs, is dropped.
func (s *Service) takeEditNote(session string) string {
	now := s.now()
	s.edits.mu.Lock()
	held := s.edits.held[session]
	delete(s.edits.held, session)
	s.edits.mu.Unlock()
	held = slices.DeleteFunc(held, func(o overlap) bool { return now.Sub(o.at) > editWindow })
	if len(held) == 0 {
		return ""
	}
	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		slog.Warn("relay: read sessions for an edit-overlap note", "session", session, "err", err)
		return ""
	}
	how := editCLI
	if self, ok := findSession(projects, session); ok && s.offersTools(self.Kind) {
		how = "send_to_session"
	}
	var notes []string
	for _, group := range byEditor(held) {
		other, ok := findSession(projects, group.session)
		if !ok || !s.runsAgent(other.ID, other.Kind) {
			continue
		}
		notes = append(notes, editEnvelope(s.lang(), other, group, how, now).String())
	}
	return strings.Join(notes, "\n\n")
}

// editGroup is the paths one other session edited, and its newest edit of them.
type editGroup struct {
	session string
	paths   []string
	newest  time.Time
}

// byEditor groups overlaps by the session that made them, in the order each
// session first appears, every path once.
func byEditor(held []overlap) []editGroup {
	var groups []editGroup
	for _, o := range held {
		i := slices.IndexFunc(groups, func(g editGroup) bool { return g.session == o.session })
		if i < 0 {
			groups = append(groups, editGroup{session: o.session})
			i = len(groups) - 1
		}
		g := &groups[i]
		if !slices.Contains(g.paths, o.path) {
			g.paths = append(g.paths, o.path)
		}
		if o.at.After(g.newest) {
			g.newest = o.at
		}
	}
	return groups
}

func editEnvelope(lang prompt.Lang, other store.Session, g editGroup, how string, now time.Time) Envelope {
	text := prompt.For(lang)
	ago := text.EditJustNow
	if minutes := int(now.Sub(g.newest).Round(time.Minute) / time.Minute); minutes > 0 {
		ago = fmt.Sprintf(text.EditAgo, fmt.Sprintf(text.Pick(minutes, text.LateMinutes), minutes))
	}
	summary := fmt.Sprintf(text.EditOverlap, other.Label, strings.Join(g.paths, ", "), ago, how)
	env := noticeEnvelope(TopicEditOverlap, summary, "")
	env.From, env.Session = other.Label, other.ID
	return env
}

func findSession(projects []store.Project, id string) (store.Session, bool) {
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return sess, true
			}
		}
	}
	return store.Session{}, false
}
