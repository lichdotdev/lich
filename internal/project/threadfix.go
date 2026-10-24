package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// A review thread handed to a session (the Pulls screen's Send to session) is
// answered on GitHub at once and resolved once the pull request moves: the
// first head after the send is taken as the fix. Resolving at the send would
// mark a thread done before any code changed; waiting for the push trades that
// for the opposite risk, an unrelated push resolving it (docs/ceilings.md).
//
// The replies are English on purpose: the reviewer reading them is someone
// else, whose language lich has no setting for.
const (
	threadFixingReply = "Fixing this. It'll be in the next commit."
	threadFixedReply  = "Fixed in %s."
	// threadFixPoll is how often the waiting pull requests are read, one gh
	// call each, and only while something waits.
	threadFixPoll = time.Minute
	// shortSHA is the length git itself abbreviates a commit to, which GitHub
	// links in a comment.
	shortSHA = 7
	prOpen   = "OPEN"
)

// threadFixStore keeps the waiting threads across restarts; main wires the
// workspace store, and nil keeps nothing.
type threadFixStore interface {
	ThreadFixes() ([]api.ThreadFix, error)
	SaveThreadFix(api.ThreadFix) error
	DeleteThreadFix(threadID string) error
}

// SetThreadFixes wires where the waiting threads are kept.
func (s *Service) SetThreadFixes(store threadFixStore) { s.fixes = store }

type prTip struct {
	HeadRefOID string `json:"headRefOid"`
	State      string `json:"state"`
}

type prKey struct {
	path   string
	number int
}

func (s *Service) readTip(path string, number int) (prTip, error) {
	out, err := s.gh(prReadTimeout, path, prArgs("view", number, "--json", "headRefOid,state")...)
	if err != nil {
		return prTip{}, err
	}
	var head prTip
	if err := json.Unmarshal(out, &head); err != nil {
		return prTip{}, fmt.Errorf("could not read the pull request's head: %w", err)
	}
	return head, nil
}

// AwaitThreadFix answers a thread just handed to a session and has it wait for
// the pull request's next push. The head is read here rather than taken from
// the window, whose read may predate a push that would resolve the thread at
// once.
func (s *Service) AwaitThreadFix(ctx context.Context, opts api.ThreadFixOptions) (api.None, error) {
	if opts.ThreadID == "" || opts.Number <= 0 || opts.CommentID == 0 {
		return api.None{}, errors.New("a pull request, a thread and one of its comments are required")
	}
	if s.fixes == nil {
		return api.None{}, errors.New("nowhere to keep the thread while it waits")
	}
	head, err := s.readTip(opts.Path, opts.Number)
	if err != nil {
		return api.None{}, err
	}
	if head.State != prOpen {
		return api.None{}, errors.New("the pull request is not open")
	}
	reply := api.PRCommentBodyOptions{Path: opts.Path, Number: opts.Number, CommentID: opts.CommentID, Body: threadFixingReply}
	if _, err := s.ReplyToReviewThread(ctx, reply); err != nil {
		return api.None{}, err
	}
	fix := api.ThreadFix{ThreadID: opts.ThreadID, Path: opts.Path, Number: opts.Number, CommentID: opts.CommentID, Head: head.HeadRefOID}
	if err := s.fixes.SaveThreadFix(fix); err != nil {
		return api.None{}, err
	}
	s.emitThreadFixes()
	return api.None{}, nil
}

// ThreadFixes is every thread waiting for its fix.
func (s *Service) ThreadFixes(_ context.Context, _ api.None) ([]api.ThreadFix, error) {
	if s.fixes == nil {
		return nil, nil
	}
	return s.fixes.ThreadFixes()
}

// RunThreadFixWatch settles the waiting threads every threadFixPoll, for as
// long as lich runs.
func (s *Service) RunThreadFixWatch() {
	ticker := time.NewTicker(threadFixPoll)
	defer ticker.Stop()
	for range ticker.C {
		s.checkThreadFixes()
	}
}

// checkThreadFixes reads each waiting pull request once and settles its
// threads. A read or a write that fails leaves the thread waiting for the next
// pass.
func (s *Service) checkThreadFixes() {
	fixes, err := s.ThreadFixes(context.Background(), api.None{})
	if err != nil {
		slog.Warn("thread fixes: list", "err", err)
		return
	}
	heads := map[prKey]prTip{}
	unread := map[prKey]bool{}
	changed := false
	for _, f := range fixes {
		key := prKey{f.Path, f.Number}
		if unread[key] {
			continue
		}
		head, ok := heads[key]
		if !ok {
			if head, err = s.waitingHead(f); err != nil {
				slog.Warn("thread fixes: read the pull request", "path", f.Path, "number", f.Number, "err", err)
				unread[key] = true
				continue
			}
			heads[key] = head
		}
		settled, err := s.settleThreadFix(f, head)
		if err != nil {
			slog.Warn("thread fixes: settle", "thread", f.ThreadID, "err", err)
		}
		changed = changed || settled
	}
	if changed {
		s.emitThreadFixes()
	}
}

// waitingHead reads f's pull request, as closed when its checkout is gone:
// gh has nothing left to run in, and nothing will ever push from it.
func (s *Service) waitingHead(f api.ThreadFix) (prTip, error) {
	if _, err := os.Stat(f.Path); errors.Is(err, fs.ErrNotExist) {
		return prTip{}, nil
	}
	return s.readTip(f.Path, f.Number)
}

// settleThreadFix resolves f once its pull request moved past the head it was
// sent at, and drops it once the pull request closed; it reports whether f
// stopped waiting. Resolving goes first: a reply that fails after it is
// retried with a resolve that changes nothing, never a second reply.
func (s *Service) settleThreadFix(f api.ThreadFix, head prTip) (bool, error) {
	if head.State != prOpen {
		return true, s.fixes.DeleteThreadFix(f.ThreadID)
	}
	if head.HeadRefOID == f.Head {
		return false, nil
	}
	if _, err := s.ResolveReviewThread(context.Background(), api.ResolveThreadOptions{Path: f.Path, ThreadID: f.ThreadID, Resolved: true}); err != nil {
		return false, err
	}
	sha := head.HeadRefOID[:min(shortSHA, len(head.HeadRefOID))]
	reply := api.PRCommentBodyOptions{Path: f.Path, Number: f.Number, CommentID: f.CommentID, Body: fmt.Sprintf(threadFixedReply, sha)}
	if _, err := s.ReplyToReviewThread(context.Background(), reply); err != nil {
		return false, err
	}
	return true, s.fixes.DeleteThreadFix(f.ThreadID)
}

func (s *Service) emitThreadFixes() {
	if s.emit == nil {
		return
	}
	fixes, err := s.ThreadFixes(context.Background(), api.None{})
	if err != nil {
		slog.Warn("thread fixes: list", "err", err)
		return
	}
	events.Send(s.emit, api.ThreadFixesEvent, fixes)
}
