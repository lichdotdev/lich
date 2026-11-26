package spawn

import (
	"context"
	"errors"
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
)

// forkedConversation is the conversation a fork branches: the caller's own,
// on a provider that can branch one (providers.SupportsFork), in the caller's
// project. A fork runs the caller's provider and starts on the conversation's
// own model and effort, since lich passes neither on a resume, so naming any
// is refused rather than dropped.
func (s *Service) forkedConversation(opts OpenOptions) (string, error) {
	if opts.Kind != "" || opts.Model != "" || opts.Effort != "" || opts.Ultracode {
		return "", errors.New(
			"a fork carries on your own conversation on your own agent, model and effort: " +
				"leave kind, model, effort and ultracode out, or open the session without fork")
	}
	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		return "", fmt.Errorf("read the workspace: %w", err)
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID != opts.From {
				continue
			}
			if opts.Project != "" && opts.Project != p.Name && opts.Project != p.Path {
				return "", errors.New("a fork opens in your own project: leave the project out")
			}
			return s.forkable(p, sess)
		}
	}
	return "", errors.New(
		"a fork branches the conversation of the session asking for it, and this is not " +
			"running in a session lich holds: run it from inside the session to fork")
}

func (s *Service) forkable(p store.Project, sess store.Session) (string, error) {
	if !providers.SupportsFork(sess.Kind) {
		return "", fmt.Errorf(
			"%s cannot fork a conversation, only resume one: open the session without fork", kindName(sess.Kind))
	}
	cwd := p.Path
	if sess.Path != "" {
		cwd = sess.Path
	}
	if sess.ProviderSessionID == "" || !s.term.Resumable(sess.Kind, sess.ProviderSessionID, cwd) {
		return "", fmt.Errorf(
			"%q has no conversation %s can branch yet: open the session without fork",
			sess.Label, kindName(sess.Kind))
	}
	return sess.ProviderSessionID, nil
}

// markFork records that session id branches a conversation, which draws it
// as its terminal: only the terminal's resume flag branches one. Held like an
// override's failure, for the reason recordOverrides gives.
func (s *Service) markFork(id, label, fork string) error {
	if fork == "" {
		return nil
	}
	if _, err := s.sessions.MarkForked(context.Background(), api.SessionIDOptions{ID: id}); err != nil {
		return fmt.Errorf("session %q is open, but it could not be marked a fork: %w", label, err)
	}
	return nil
}
