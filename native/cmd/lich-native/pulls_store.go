package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/backend"
)

// pendingRecheck is how often a pull request still settling is read again
// while it is on screen.
const pendingRecheck = 10 * time.Second

// pullsRead is one backend read: its last answer, and whether a newer one is
// owed or on its way.
type pullsRead[T any] struct {
	val     T
	err     string
	loaded  bool
	loading bool
	stale   bool
	at      time.Time
}

// pullsStore is the window's pull request reads for the Pulls screen. A write
// through lich comes back as PRChangedEvent and marks everything stale; the
// screen reads what it draws each frame, which fetches what is missing or
// stale, one read per key at a time.
type pullsStore struct {
	ctx        context.Context
	client     *backend.Client
	invalidate func()

	mu sync.Mutex
	// staledAt is when every read was last owed a newer answer.
	staledAt  time.Time
	lists     map[string]*pullsRead[[]backend.PRSummary]
	details   map[string]*pullsRead[*backend.PRDetail]
	rules     map[string]*pullsRead[*backend.BranchRules]
	conflicts map[string]*pullsRead[[]string]
	checkouts map[string]*pullsRead[[]backend.Worktree]
	diffs     map[string]*pullsRead[string]
	talks     map[string]*pullsRead[*backend.PRConversation]
}

func newPullsStore(ctx context.Context, c *backend.Client, invalidate func()) *pullsStore {
	return &pullsStore{
		ctx: ctx, client: c, invalidate: invalidate,
		lists:     map[string]*pullsRead[[]backend.PRSummary]{},
		details:   map[string]*pullsRead[*backend.PRDetail]{},
		rules:     map[string]*pullsRead[*backend.BranchRules]{},
		conflicts: map[string]*pullsRead[[]string]{},
		checkouts: map[string]*pullsRead[[]backend.Worktree]{},
		diffs:     map[string]*pullsRead[string]{},
		talks:     map[string]*pullsRead[*backend.PRConversation]{},
	}
}

// read answers key's last value and starts load when there is none yet, or a
// newer one is owed and nothing is fetching it.
func read[T any](s *pullsStore, entries map[string]*pullsRead[T], key string, load func() (T, error)) pullsRead[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := entries[key]
	if !ok {
		e = &pullsRead[T]{}
		entries[key] = e
	}
	if (!e.loaded || e.stale) && !e.loading {
		e.loading, e.stale = true, false
		go func() {
			val, err := load()
			s.mu.Lock()
			e.loading, e.loaded, e.at = false, true, time.Now()
			e.val, e.err = val, ""
			if err != nil {
				e.err = backend.ErrorText(err)
			}
			s.mu.Unlock()
			s.invalidate()
		}()
	}
	return *e
}

func (s *pullsStore) lastStale() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.staledAt
}

func staleAll[T any](entries map[string]*pullsRead[T]) {
	for _, e := range entries {
		e.stale = true
	}
}

// stale owes every read a newer answer: a write landed, the user asked, the
// screen or the window came back into view, or prRecheck passed.
func (s *pullsStore) stale() {
	s.mu.Lock()
	s.staledAt = time.Now()
	staleAll(s.lists)
	staleAll(s.details)
	staleAll(s.rules)
	staleAll(s.conflicts)
	staleAll(s.checkouts)
	staleAll(s.diffs)
	staleAll(s.talks)
	s.mu.Unlock()
	s.invalidate()
}

func (s *pullsStore) list(path string, state pullsState) pullsRead[[]backend.PRSummary] {
	return read(s, s.lists, path+"\x00"+string(state), func() ([]backend.PRSummary, error) {
		return s.client.ListPullRequests(s.ctx, path, string(state))
	})
}

// detail reads pull request number, again every pendingRecheck while it is
// still settling (stillSettling).
func (s *pullsStore) detail(path string, number int) pullsRead[*backend.PRDetail] {
	key := fmt.Sprint(path, "\x00", number)
	s.mu.Lock()
	if e, ok := s.details[key]; ok && e.val != nil && stillSettling(*e.val) && time.Since(e.at) >= pendingRecheck {
		e.stale = true
	}
	s.mu.Unlock()
	return read(s, s.details, key, func() (*backend.PRDetail, error) {
		return s.client.PullRequestDetail(s.ctx, path, number)
	})
}

// stillSettling is a pull request whose answer GitHub has not finished: a check
// still running, or mergeability it reads UNKNOWN while it recomputes after a
// push to the branch or its base. Nothing announces the end of either.
func stillSettling(d backend.PRDetail) bool {
	if d.Checks.Pending > 0 {
		return true
	}
	return d.State == "OPEN" && (d.Mergeable == "UNKNOWN" || d.MergeStateStatus == "UNKNOWN")
}

func (s *pullsStore) branchRules(path, base string) *backend.BranchRules {
	return read(s, s.rules, path+"\x00"+base, func() (*backend.BranchRules, error) {
		return s.client.BranchRules(s.ctx, path, base)
	}).val
}

func (s *pullsStore) conflictFiles(path string, d *backend.PRDetail) pullsRead[[]string] {
	return read(s, s.conflicts, d.URL, func() ([]string, error) {
		return s.client.PullRequestConflicts(s.ctx, path, d.Number, d.BaseRefName, d.URL)
	})
}

func (s *pullsStore) diff(path string, number int) pullsRead[string] {
	return read(s, s.diffs, fmt.Sprint(path, "\x00", number), func() (string, error) {
		return s.client.PullRequestDiff(s.ctx, path, number)
	})
}

func (s *pullsStore) conversation(path string, number int) pullsRead[*backend.PRConversation] {
	return read(s, s.talks, fmt.Sprint(path, "\x00", number), func() (*backend.PRConversation, error) {
		return s.client.PullRequestConversation(s.ctx, path, number)
	})
}

func (s *pullsStore) checkoutsOf(path string) []backend.Worktree {
	return read(s, s.checkouts, path, func() ([]backend.Worktree, error) {
		return s.client.ListCheckouts(s.ctx, path)
	}).val
}

// freshCheckouts reads path's checkouts now, for an action that must not act
// on a directory removed since the screen was drawn.
func (s *pullsStore) freshCheckouts(path string) ([]backend.Worktree, error) {
	list, err := s.client.ListCheckouts(s.ctx, path)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.checkouts[path] = &pullsRead[[]backend.Worktree]{val: list, loaded: true, at: time.Now()}
	s.mu.Unlock()
	return list, nil
}
