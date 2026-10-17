package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
)

// pendingRecheck is use-pull-request-detail.ts's poll: a pull request with a
// check still running is read again this often while it is on screen.
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
	client     *lichclient.Client
	invalidate func()

	mu        sync.Mutex
	lists     map[string]*pullsRead[[]lichclient.PRSummary]
	details   map[string]*pullsRead[*lichclient.PRDetail]
	rules     map[string]*pullsRead[*lichclient.BranchRules]
	conflicts map[string]*pullsRead[[]string]
	checkouts map[string]*pullsRead[[]lichclient.Worktree]
	diffs     map[string]*pullsRead[string]
	talks     map[string]*pullsRead[*lichclient.PRConversation]
}

func newPullsStore(ctx context.Context, c *lichclient.Client, invalidate func()) *pullsStore {
	return &pullsStore{
		ctx: ctx, client: c, invalidate: invalidate,
		lists:     map[string]*pullsRead[[]lichclient.PRSummary]{},
		details:   map[string]*pullsRead[*lichclient.PRDetail]{},
		rules:     map[string]*pullsRead[*lichclient.BranchRules]{},
		conflicts: map[string]*pullsRead[[]string]{},
		checkouts: map[string]*pullsRead[[]lichclient.Worktree]{},
		diffs:     map[string]*pullsRead[string]{},
		talks:     map[string]*pullsRead[*lichclient.PRConversation]{},
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
				e.err = lichclient.ErrorText(err)
			}
			s.mu.Unlock()
			s.invalidate()
		}()
	}
	return *e
}

func staleAll[T any](entries map[string]*pullsRead[T]) {
	for _, e := range entries {
		e.stale = true
	}
}

// stale owes every read a newer answer: a write landed, the user asked, or the
// screen came back into view.
func (s *pullsStore) stale() {
	s.mu.Lock()
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

func (s *pullsStore) list(path string, state pullsState) pullsRead[[]lichclient.PRSummary] {
	return read(s, s.lists, path+"\x00"+string(state), func() ([]lichclient.PRSummary, error) {
		return s.client.ListPullRequests(s.ctx, path, string(state))
	})
}

// detail reads pull request number, again every pendingRecheck while one of
// its checks is still running.
func (s *pullsStore) detail(path string, number int) pullsRead[*lichclient.PRDetail] {
	key := fmt.Sprint(path, "\x00", number)
	s.mu.Lock()
	if e, ok := s.details[key]; ok && e.val != nil && e.val.Checks.Pending > 0 && time.Since(e.at) >= pendingRecheck {
		e.stale = true
	}
	s.mu.Unlock()
	return read(s, s.details, key, func() (*lichclient.PRDetail, error) {
		return s.client.PullRequestDetail(s.ctx, path, number)
	})
}

func (s *pullsStore) branchRules(path, base string) *lichclient.BranchRules {
	return read(s, s.rules, path+"\x00"+base, func() (*lichclient.BranchRules, error) {
		return s.client.BranchRules(s.ctx, path, base)
	}).val
}

func (s *pullsStore) conflictFiles(path string, d *lichclient.PRDetail) pullsRead[[]string] {
	return read(s, s.conflicts, d.URL, func() ([]string, error) {
		return s.client.PullRequestConflicts(s.ctx, path, d.Number, d.BaseRefName, d.URL)
	})
}

func (s *pullsStore) diff(path string, number int) pullsRead[string] {
	return read(s, s.diffs, fmt.Sprint(path, "\x00", number), func() (string, error) {
		return s.client.PullRequestDiff(s.ctx, path, number)
	})
}

func (s *pullsStore) conversation(path string, number int) pullsRead[*lichclient.PRConversation] {
	return read(s, s.talks, fmt.Sprint(path, "\x00", number), func() (*lichclient.PRConversation, error) {
		return s.client.PullRequestConversation(s.ctx, path, number)
	})
}

func (s *pullsStore) checkoutsOf(path string) []lichclient.Worktree {
	return read(s, s.checkouts, path, func() ([]lichclient.Worktree, error) {
		return s.client.ListCheckouts(s.ctx, path)
	}).val
}

// freshCheckouts reads path's checkouts now, for an action that must not act
// on a directory removed since the screen was drawn.
func (s *pullsStore) freshCheckouts(path string) ([]lichclient.Worktree, error) {
	list, err := s.client.ListCheckouts(s.ctx, path)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.checkouts[path] = &pullsRead[[]lichclient.Worktree]{val: list, loaded: true, at: time.Now()}
	s.mu.Unlock()
	return list, nil
}
