package project

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// The pull request of each checkout's branch has one reader, here. The cards'
// badges hear every read as api.BranchPullRequestEvent, and a merge made
// outside lich (on github.com, with gh in a terminal, by an agent) is told to
// the agents in its checkout from the same reads, the way the Pulls screen's
// merge is.

// branchPRTick is how often the watch reads the pull request of every checkout
// it follows: one gh call per checkout, for a badge and a merge notice that
// read as fine a minute late.
const branchPRTick = time.Minute

// prMerged is the state gh reports a merged pull request in.
const prMerged = "MERGED"

// branchPRs is what the watch last read at each checkout, and which reads are
// running. A path in last was read at least once; its value is nil for a
// branch with no pull request.
type branchPRs struct {
	mu      sync.Mutex
	last    map[string]*PullRequest
	reading map[string]bool
	again   map[string]bool
}

// SetAgentCheckouts wires the checkouts an agent runs in, which the watch reads
// every tick: a pull request opens there whether or not a window is looking.
func (s *Service) SetAgentCheckouts(checkouts func() []string) { s.agentCheckouts = checkouts }

// SetAnnounceMerge wires who tells a checkout's agents that its pull request
// merged (internal/relay AnnounceMerge).
func (s *Service) SetAnnounceMerge(announce func(context.Context, api.AnnounceMergeOptions) (api.None, error)) {
	s.announceMerge = announce
}

// RecheckPullRequest reads the pull request of opts.Path's branch again, behind
// the call: the answer arrives as api.BranchPullRequestEvent. A window asks
// when it has seen the branch or the head move, a turn end, or anything else
// gh would answer differently after.
func (s *Service) RecheckPullRequest(_ context.Context, opts api.PathOptions) (api.None, error) {
	go s.readBranchPR(opts.Path)
	return api.None{}, nil
}

// RunBranchPRWatch reads the followed checkouts every branchPRTick, for as long
// as lich runs.
func (s *Service) RunBranchPRWatch() {
	ticker := time.NewTicker(branchPRTick)
	defer ticker.Stop()
	for range ticker.C {
		for _, path := range s.followedCheckouts() {
			s.readBranchPR(path)
		}
	}
}

// followedCheckouts is every checkout an agent runs in, and every one whose
// last read found an open pull request, which may merge or close with nobody
// in the checkout. A checkout with neither is read only when a window asks.
func (s *Service) followedCheckouts() []string {
	var paths []string
	if s.agentCheckouts != nil {
		paths = s.agentCheckouts()
	}
	s.prs.mu.Lock()
	defer s.prs.mu.Unlock()
	for path, pr := range s.prs.last {
		if pr != nil && pr.State == prOpen && !slices.Contains(paths, path) {
			paths = append(paths, path)
		}
	}
	return paths
}

// readBranchPR reads path's pull request, one read at a time per path. One
// asked for while another runs is read again once it ends, since the branch
// may have moved under the first.
func (s *Service) readBranchPR(path string) {
	if !s.prs.claim(path) {
		return
	}
	for {
		s.recordBranchPR(path)
		if !s.prs.release(path) {
			return
		}
	}
}

// recordBranchPR reads path's pull request, reports it and announces a merge
// the read before did not see. A failed read reports nothing and keeps what the
// last one saw, except for a checkout that is gone, which is forgotten.
func (s *Service) recordBranchPR(path string) {
	pr, err := s.BranchPullRequest(path)
	if err != nil {
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			s.prs.forget(path)
			return
		}
		slog.Debug("project: read the pull request of a checkout", "path", path, "err", err)
		return
	}
	merged := s.prs.record(path, pr)
	if s.emit != nil {
		events.Send(s.emit, api.BranchPullRequestEvent, api.BranchPullRequest{Path: path, PR: pr})
	}
	if !merged || s.announceMerge == nil {
		return
	}
	opts := api.AnnounceMergeOptions{Checkout: path, Number: pr.Number, Title: pr.Title, Branch: pr.HeadRefName, Base: pr.BaseRefName}
	if _, err := s.announceMerge(context.Background(), opts); err != nil {
		slog.Warn("project: announce a merge made outside lich", "path", path, "err", err)
	}
}

// BranchPullRequest returns the pull request of the branch checked out at path
// whatever its state, nil when the branch has none.
func (s *Service) BranchPullRequest(path string) (*PullRequest, error) {
	out, err := s.gh(prLookupTimeout, path, "pr", "view", "--json", "number,url,title,state,headRefName,baseRefName")
	if errors.Is(err, errNoPullRequest) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pr PullRequest
	if err := json.Unmarshal(out, &pr); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	// gh emits `{}` in some no-PR states.
	if pr.Number == 0 {
		return nil, nil
	}
	return &pr, nil
}

// claim marks a read of path running and answers whether the caller is the one
// to run it; otherwise the running one reads again when it ends.
func (b *branchPRs) claim(path string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.reading == nil {
		b.reading, b.again = map[string]bool{}, map[string]bool{}
	}
	if b.reading[path] {
		b.again[path] = true
		return false
	}
	b.reading[path] = true
	return true
}

// release ends a read of path and answers whether another was asked for
// meanwhile, which the caller then runs.
func (b *branchPRs) release(path string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.again[path] {
		delete(b.again, path)
		return true
	}
	delete(b.reading, path)
	return false
}

// record keeps pr as path's last read and answers whether it is the pull
// request the read before found open, now merged. One first read already
// merged is not announced: it may have merged long before lich started, and
// its agents heard of it then or never will.
func (b *branchPRs) record(path string, pr *PullRequest) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.last == nil {
		b.last = map[string]*PullRequest{}
	}
	prev := b.last[path]
	b.last[path] = pr
	return prev != nil && prev.State == prOpen && pr != nil && pr.State == prMerged && pr.Number == prev.Number
}

func (b *branchPRs) forget(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.last, path)
}
