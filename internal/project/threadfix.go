package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// A review thread handed to a session (the Pulls screen's Send to session) is
// not all a request for a change: a question or a point for the team is the
// developer's to answer. So nothing is said on GitHub at the send. The agent
// marks the commit that fixes a thread (MarkThreadFixed), and the thread is
// resolved once the pull request's head holds that commit (docs/ceilings.md).
//
// The reply is English on purpose: the reviewer reading it is someone else,
// whose language lich has no setting for.
const (
	threadFixedReply = "Fixed in %s."
	// threadFixPoll is how often the pull requests with a marked thread are
	// read, one gh call each, and only while one waits.
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

// AwaitThreadFix keeps a thread just handed to a session, for its agent to
// mark the commit that fixes it.
func (s *Service) AwaitThreadFix(_ context.Context, opts api.ThreadFixOptions) (api.None, error) {
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
	fix := api.ThreadFix{ThreadID: opts.ThreadID, Path: opts.Path, Number: opts.Number, CommentID: opts.CommentID}
	if err := s.fixes.SaveThreadFix(fix); err != nil {
		return api.None{}, err
	}
	s.emitThreadFixes()
	return api.None{}, nil
}

// MarkThreadFixed records commit as the fix of a thread handed to a session.
// Only a thread the Pulls screen sent can be marked: the send is the person's
// consent to have lich resolve and reply on it. The commit is checked in the
// pull request's checkout, which shares its objects with every worktree of it.
func (s *Service) MarkThreadFixed(_ context.Context, opts api.MarkThreadFixedOptions) (api.None, error) {
	fix, err := s.sentThread(opts.ThreadID)
	if err != nil {
		return api.None{}, err
	}
	full, err := fixCommit(fix.Path, opts.Commit)
	if err != nil {
		return api.None{}, err
	}
	fix.Commit = full
	if err := s.fixes.SaveThreadFix(fix); err != nil {
		return api.None{}, err
	}
	s.emitThreadFixes()
	return api.None{}, nil
}

// fixCommit is the full sha of commit in dir's repository. An option-shaped
// commit is refused before git sees it.
func fixCommit(dir, commit string) (string, error) {
	if commit != "" && !strings.HasPrefix(commit, "-") {
		if out, ok := gitQuiet(dir, "rev-parse", "--verify", "--quiet", commit+"^{commit}"); ok {
			return strings.TrimSpace(out), nil
		}
	}
	return "", fmt.Errorf("no commit %q in this repository: commit the fix first and pass its sha", commit)
}

func (s *Service) sentThread(id string) (api.ThreadFix, error) {
	if s.fixes == nil {
		return api.ThreadFix{}, errors.New("no review thread was handed to a session")
	}
	fixes, err := s.fixes.ThreadFixes()
	if err != nil {
		return api.ThreadFix{}, err
	}
	for _, f := range fixes {
		if f.ThreadID == id {
			return f, nil
		}
	}
	return api.ThreadFix{}, fmt.Errorf("review thread %q was not handed to a session from the Pulls screen", id)
}

// ThreadFixes is every thread handed to a session.
func (s *Service) ThreadFixes(_ context.Context, _ api.None) ([]api.ThreadFix, error) {
	if s.fixes == nil {
		return nil, nil
	}
	return s.fixes.ThreadFixes()
}

// RunThreadFixWatch settles the marked threads every threadFixPoll, for as
// long as lich runs. Its first pass reads every pull request a thread was sent
// on, so the threads nobody marked are dropped with their closed pull request
// once per launch rather than polled for as long as it stays open.
func (s *Service) RunThreadFixWatch() {
	s.checkThreadFixes(s.listThreadFixes())
	ticker := time.NewTicker(threadFixPoll)
	defer ticker.Stop()
	for range ticker.C {
		s.checkThreadFixes(markedFixes(s.listThreadFixes()))
	}
}

func (s *Service) listThreadFixes() []api.ThreadFix {
	fixes, err := s.ThreadFixes(context.Background(), api.None{})
	if err != nil {
		slog.Warn("thread fixes: list", "err", err)
	}
	return fixes
}

// markedFixes is the threads whose agent named the commit that fixes them.
func markedFixes(fixes []api.ThreadFix) []api.ThreadFix {
	var out []api.ThreadFix
	for _, f := range fixes {
		if f.Commit != "" {
			out = append(out, f)
		}
	}
	return out
}

// checkThreadFixes reads each of fixes' pull requests once and settles their
// threads. A read or a write that fails leaves the thread waiting for the next
// pass.
func (s *Service) checkThreadFixes(fixes []api.ThreadFix) {
	var err error
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

// settleThreadFix resolves f once its pull request's head holds the commit its
// agent marked, and drops it once the pull request closed; it reports whether
// f stopped waiting. A head this checkout has not fetched reads as not holding
// it, until a push from here or a fetch brings it. Resolving goes first: a
// reply that fails after it is retried with a resolve that changes nothing,
// never a second reply.
func (s *Service) settleThreadFix(f api.ThreadFix, head prTip) (bool, error) {
	if head.State != prOpen {
		return true, s.fixes.DeleteThreadFix(f.ThreadID)
	}
	if f.Commit == "" {
		return false, nil
	}
	if _, held := gitQuiet(f.Path, "merge-base", "--is-ancestor", f.Commit, head.HeadRefOID); !held {
		return false, nil
	}
	if _, err := s.ResolveReviewThread(context.Background(), api.ResolveThreadOptions{Path: f.Path, ThreadID: f.ThreadID, Resolved: true}); err != nil {
		return false, err
	}
	sha := f.Commit[:min(shortSHA, len(f.Commit))]
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
