package extensions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/system"
)

const (
	// maxNotifications is how many desktop notifications one extension raises
	// in notifyWindow; one past it is refused. lich raises one per session
	// turn at most, and an extension notifying faster than this is a loop.
	maxNotifications = 3
	notifyWindow     = time.Minute
)

var errNoWindow = errors.New("no lich window is open to show it")

// gitStatus reads what a card reads of a checkout (native gitstate.go
// refreshPath): the window's store is in the window, so the backend reads
// the same two answers it does.
func (s *Service) gitStatus(ctx context.Context, _ *installed, params extension.GitParams) (extension.GitStatusResult, error) {
	opts := api.PathOptions{Path: params.Path}
	diff, err := s.lich.Diff(ctx, opts)
	if err != nil {
		return extension.GitStatusResult{}, err
	}
	base, err := s.lich.BaseStatus(ctx, opts)
	if err != nil {
		return extension.GitStatusResult{}, err
	}
	out := extension.GitStatusResult{
		Branch: diff.Branch, Head: diff.Head, Files: diff.Files, Added: diff.Added, Deleted: diff.Deleted,
		Conflicts: []string{},
	}
	if base != nil {
		out.Base, out.Behind = base.Base, base.Behind
		if base.Conflicts != nil {
			out.Conflicts = base.Conflicts
		}
	}
	return out, nil
}

func (s *Service) gitPullRequest(ctx context.Context, _ *installed, params extension.GitParams) (extension.PullRequestResult, error) {
	pr, err := s.lich.PullRequest(ctx, api.PathOptions{Path: params.Path})
	if err != nil || pr == nil {
		return extension.PullRequestResult{}, err
	}
	return extension.PullRequestResult{PullRequest: &extension.PullRequest{
		Number: pr.Number, URL: pr.URL, Title: pr.Title, Head: pr.HeadRefName, Base: pr.BaseRefName,
	}}, nil
}

func (s *Service) notify(ctx context.Context, x *installed, params extension.NotifyParams) (extension.NoParams, error) {
	if err := s.takeNotification(x, time.Now()); err != nil {
		return extension.NoParams{}, err
	}
	_, err := s.lich.Notify(ctx, api.NotifyOptions{Summary: params.Title, Detail: params.Body})
	return extension.NoParams{}, err
}

// takeNotification counts one notification against x's bound, or refuses it.
func (s *Service) takeNotification(x *installed, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := x.notified[:0]
	for _, at := range x.notified {
		if now.Sub(at) < notifyWindow {
			recent = append(recent, at)
		}
	}
	x.notified = recent
	if len(recent) >= maxNotifications {
		return fmt.Errorf("%d notifications in the last %s, the most one extension may raise", maxNotifications, notifyWindow)
	}
	x.notified = append(x.notified, now)
	return nil
}

func (s *Service) showCode(ctx context.Context, _ *installed, params extension.ShowCodeParams) (extension.ShownCode, error) {
	if !s.lich.Attached() {
		return extension.ShownCode{}, errNoWindow
	}
	shown, err := s.lich.ShowCode(ctx, api.ShowCodeOptions{
		ID: params.Session, Path: params.Path, Line: params.Line, EndLine: params.EndLine, Comment: params.Comment,
	})
	if err != nil {
		return extension.ShownCode{}, err
	}
	return extension.ShownCode{Session: shown.SessionID, Path: shown.Rel, Line: shown.Line, EndLine: shown.EndLine}, nil
}

// openURL checks the session is live first: the browser starts one Chromium
// for any owner it is given.
func (s *Service) openURL(ctx context.Context, _ *installed, params extension.OpenURLParams) (extension.OpenedURL, error) {
	if !s.lich.Attached() {
		return extension.OpenedURL{}, errNoWindow
	}
	if err := system.ValidateExternalURL(params.URL); err != nil {
		return extension.OpenedURL{}, err
	}
	if err := s.liveSession(ctx, params.Session); err != nil {
		return extension.OpenedURL{}, err
	}
	page, err := s.lich.OpenURL(ctx, api.BrowserOpenOptions{ID: params.Session, URL: params.URL})
	if err != nil {
		return extension.OpenedURL{}, err
	}
	return extension.OpenedURL{URL: page.URL, Title: page.Title}, nil
}

func (s *Service) liveSession(ctx context.Context, id string) error {
	peers, err := s.lich.Peers(ctx, api.PeersOptions{})
	if err != nil {
		return err
	}
	for _, p := range peers {
		if p.ID == id {
			return nil
		}
	}
	return fmt.Errorf("session %q is not running", id)
}
