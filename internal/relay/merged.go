package relay

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
)

// AnnounceMerge tells every agent running in opts.Checkout that the pull request on
// its branch was merged from lich's own screen. An agent keeps notes on the work
// it did there, and without this the last thing they say about it is that it is
// waiting on a merge, which a later session reads back as true long after the
// checkout is gone. The session that opened one of those agents hears it too,
// wherever it runs: it handed the work out and keeps the same kind of notes on
// it.
//
// Only provider sessions hear it: a terminal card in the same checkout is a
// shell, where a typed notice would run as a command. A card with no process
// running has nobody to tell and is skipped, as a parked one is.
//
// Delivery runs in the background and a failure is only logged: the merge it
// reports already happened, and nothing the caller could do with the error would
// undo it. The error returned is the workspace failing to load, before anyone
// was told anything.
func (s *Service) AnnounceMerge(ctx context.Context, opts api.AnnounceMergeOptions) (api.None, error) {
	projects, err := s.sessions.LoadState(ctx, api.None{})
	if err != nil {
		return api.None{}, fmt.Errorf("read sessions: %w", err)
	}
	lang := s.lang()
	message := mergeNotice(lang, opts.Number, opts.Title, opts.Branch, opts.Base)
	summary := fmt.Sprintf(prompt.For(lang).MergeSummary, opts.Number, opts.Base)
	note := &Notification{Status: NotifyCompleted, Summary: summary}
	for _, sess := range s.mergeAudience(projects, opts.Checkout) {
		go func(id, label string) {
			if err := s.deliver(id, message, note); err != nil {
				slog.Warn("relay: merge notice not delivered", "session", label, "err", err)
			}
		}(sess.ID, sess.Label)
	}
	return api.None{}, nil
}

// mergeAudience is the live provider sessions running in checkout, and the live
// provider sessions that opened any session there, each once. An opener hears
// it even when the session it opened is parked: the work it handed out is
// done either way.
func (s *Service) mergeAudience(projects []store.Project, checkout string) []store.Session {
	byID := map[string]store.Session{}
	var inCheckout []store.Session
	for _, p := range projects {
		for _, sess := range p.Sessions {
			byID[sess.ID] = sess
			if sessionCwd(sess.Path, p.Path) == checkout {
				inCheckout = append(inCheckout, sess)
			}
		}
	}
	told := map[string]bool{}
	var audience []store.Session
	add := func(sess store.Session) {
		if !told[sess.ID] && s.runsAgent(sess.ID, sess.Kind) {
			told[sess.ID] = true
			audience = append(audience, sess)
		}
	}
	for _, sess := range inCheckout {
		add(sess)
	}
	for _, sess := range inCheckout {
		if origin, ok := byID[sess.OriginSessionID]; ok {
			add(origin)
		}
	}
	return audience
}

// runsAgent is whether a session is a live provider session.
func (s *Service) runsAgent(id, kind string) bool {
	if !providers.Known(kind) {
		return false
	}
	live, err := s.term.Live(context.Background(), api.SessionIDOptions{ID: id})
	if err != nil {
		slog.Warn("relay: read whether a session runs, leaving it out of the merge notice", "session", id, "err", err)
		return false
	}
	return live
}

// sessionCwd is the directory a session runs in: its worktree, or the project's
// own directory for a session that names none.
func sessionCwd(sessionPath, projectPath string) string {
	if sessionPath == "" {
		return projectPath
	}
	return sessionPath
}

// mergeNotice is what the agent reads. It says the work is done and asks for
// nothing beyond keeping its record straight: a notice that read as a task would
// start one (deleting the branch, pulling the base) in a checkout the user may
// be about to remove.
func mergeNotice(lang prompt.Lang, number int, title, branch, base string) string {
	return fmt.Sprintf(prompt.For(lang).MergeNotice, number, sanitize(title), branch, base)
}
