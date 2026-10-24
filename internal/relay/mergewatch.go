package relay

import (
	"context"
	"log/slog"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/store"
)

// mergeWatchTick is how often the merge watch reads the pull request of every
// checkout an agent runs in: one gh call per checkout, for a notice that
// reads as fine a minute late.
const mergeWatchTick = time.Minute

// prOpen and prMerged are the states gh reports a pull request in.
const (
	prOpen   = "OPEN"
	prMerged = "MERGED"
)

// BranchPullRequest reads the pull request of the branch checked out at path,
// whatever its state, nil for none (internal/project implements it).
type BranchPullRequest func(path string) (*api.PRDetail, error)

// RunMergeWatch announces a merge made outside lich (on github.com, with gh in
// a terminal, by an agent) the way the Pulls screen announces its own, until
// the process ends. Started once at launch: nothing tells lich of such a
// merge, so it reads every checkout an agent runs in each mergeWatchTick.
func (s *Service) RunMergeWatch(read BranchPullRequest) {
	ticker := time.NewTicker(mergeWatchTick)
	defer ticker.Stop()
	for range ticker.C {
		s.watchMerges(read)
	}
}

// watchMerges reads each checkout's pull request once and announces the ones
// that merged since the last read. A failed read (gh missing, not a GitHub
// repo) is left for the next tick and keeps what the last one saw.
func (s *Service) watchMerges(read BranchPullRequest) {
	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		slog.Warn("relay: read sessions for the merge watch", "err", err)
		return
	}
	for _, checkout := range s.agentCheckouts(projects) {
		pr, err := read(checkout)
		if err != nil {
			slog.Debug("relay: read the pull request of a checkout", "checkout", checkout, "err", err)
			continue
		}
		if !s.mergedSinceLastRead(checkout, pr) {
			continue
		}
		opts := api.AnnounceMergeOptions{Checkout: checkout, Number: pr.Number, Title: pr.Title, Branch: pr.HeadRefName, Base: pr.BaseRefName}
		if _, err := s.AnnounceMerge(context.Background(), opts); err != nil {
			slog.Warn("relay: announce a merge made outside lich", "checkout", checkout, "err", err)
		}
	}
}

// agentCheckouts is every checkout a live provider session runs in, each once.
func (s *Service) agentCheckouts(projects []store.Project) []string {
	seen := map[string]bool{}
	var checkouts []string
	for _, p := range projects {
		for _, sess := range p.Sessions {
			c := sessionCwd(sess.Path, p.Path)
			if !seen[c] && s.runsAgent(sess.ID, sess.Kind) {
				seen[c] = true
				checkouts = append(checkouts, c)
			}
		}
	}
	return checkouts
}

// mergedSinceLastRead records the pull request read at checkout and answers
// whether it is the one the last read found open, now merged. One first read
// already merged is not announced: it may have merged long before lich
// started, and its agents have heard of it then or never will.
func (s *Service) mergedSinceLastRead(checkout string, pr *api.PRDetail) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	was, open := s.openPRs[checkout]
	if pr != nil && pr.State == prOpen {
		s.openPRs[checkout] = pr.Number
		return false
	}
	delete(s.openPRs, checkout)
	return open && pr != nil && pr.State == prMerged && pr.Number == was
}
