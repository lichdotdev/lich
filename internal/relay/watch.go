package relay

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/store"
)

// A watch types a session's prompt when something happens on the pull request
// of its branch. The check is plain code on the read the pull request watch
// already makes (internal/project, branchpr.go), so nothing changing costs no
// turn. Each read compares what is true now with what already fired (Seen):
// a restart, a closed window or a busy session lose nothing, and a push before
// delivery replaces the commit that was going to be reported.

const (
	seenFailed = "f:"
	seenPassed = "p:"
	seenReview = "r:"
	// shortHead is how much of a commit the notice names.
	shortHead = 12
)

// WatchesCheckout is whether any open session in checkout holds a watch, which
// is what makes its pull request worth the extra read.
func (s *Service) WatchesCheckout(checkout string) bool {
	return len(s.watchersOf(checkout)) > 0
}

func (s *Service) watchersOf(checkout string) []store.Session {
	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		slog.Warn("relay: read sessions for their watches", "err", err)
		return nil
	}
	var watching []store.Session
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.Watch.On != "" && sessionCwd(sess.Path, p.Path) == checkout {
				watching = append(watching, sess)
			}
		}
	}
	return watching
}

// ReadPullRequest checks every watch in the read's checkout against it.
func (s *Service) ReadPullRequest(read api.PRRead) {
	for _, sess := range s.watchersOf(read.Path) {
		if read.Open {
			s.stepWatch(sess, read)
			continue
		}
		logSaveWatch(sess.Label, s.saveWatch(sess.ID, api.SessionWatch{}))
	}
}

// stepWatch fires the watch if what it waits for is true and has not fired yet.
// A session that cannot take the prompt now is left as it is, and the next read
// asks again.
func (s *Service) stepWatch(sess store.Session, read api.PRRead) {
	w := sess.Watch
	mark, due := watchMark(w, read)
	if w.On == api.WatchReview && !strings.HasPrefix(w.Seen, seenReview) {
		w.Seen = mark
		logSaveWatch(sess.Label, s.saveWatch(sess.ID, w))
		return
	}
	if !due || mark == w.Seen {
		return
	}
	if w.PR != read.Number {
		w.PR, w.Runs = read.Number, 0
	}
	if w.Runs >= api.WatchRunLimit {
		s.stopWatch(sess, w)
		return
	}
	if takes, err := s.takesDelivery(sess.ID); err != nil || !takes || !s.runsAgent(sess.ID, sess.Kind) {
		return
	}
	s.fireWatch(sess, w, mark, read)
}

// watchMark is what identifies the event a read reports, and whether the
// watch's condition holds. Checks are judged once none is pending, so a run
// that fails reaches the agent whole.
func watchMark(w api.SessionWatch, read api.PRRead) (string, bool) {
	settled := read.Checks.Total > 0 && read.Checks.Pending == 0
	switch w.On {
	case api.WatchChecksFailed:
		return seenFailed + read.Head, settled && read.Checks.Failed > 0
	case api.WatchChecksPassed:
		return seenPassed + read.Head, settled && read.Checks.Failed == 0
	}
	return seenReview + read.Review, read.Review != ""
}

// fireWatch records the firing before the write, for the reason deliverDue
// does: a prompt lost to a failed write can be retyped, two turns cannot be
// taken back. Only a watch on failing checks stays armed, for the next commit.
func (s *Service) fireWatch(sess store.Session, w api.SessionWatch, mark string, read api.PRRead) {
	text := watchNotice(s.lang(), w.On, read) + w.Prompt
	w.Seen = mark
	w.Runs++
	if w.On != api.WatchChecksFailed {
		w.On, w.Prompt = "", ""
	}
	if err := s.saveWatch(sess.ID, w); err != nil {
		logSaveWatch(sess.Label, err)
		return
	}
	go func() {
		if err := s.deliver(sess.ID, text, nil); err != nil {
			slog.Warn("relay: watch prompt not delivered", "session", sess.Label, "err", err)
		}
	}()
}

func (s *Service) stopWatch(sess store.Session, w api.SessionWatch) {
	stopped := api.SessionWatch{Runs: w.Runs, PR: w.PR, Seen: w.Seen}
	if err := s.saveWatch(sess.ID, stopped); err != nil {
		logSaveWatch(sess.Label, err)
		return
	}
	if s.events != nil {
		events.Send(s.events.Emit, api.SessionWatchStoppedEvent, api.SessionWatchStopped{
			ID: sess.ID, Label: sess.Label, Number: w.PR, Runs: w.Runs,
		})
	}
}

// watchNotice says what happened in front of the prompt. Only the number and
// the commit go in: a review's body and a check's log are written by others,
// and the agent reads them with gh, knowing where they came from.
func watchNotice(lang prompt.Lang, on string, read api.PRRead) string {
	text := prompt.For(lang)
	head := read.Head
	if len(head) > shortHead {
		head = head[:shortHead]
	}
	switch on {
	case api.WatchChecksFailed:
		return fmt.Sprintf(text.WatchChecksFailed, read.Number, head)
	case api.WatchChecksPassed:
		return fmt.Sprintf(text.WatchChecksPassed, read.Number, head)
	}
	return fmt.Sprintf(text.WatchReview, read.Number)
}
