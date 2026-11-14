package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/lichdotdev/lich/native/backend"
)

const (
	// gitLogPoll is how often an open panel asks whether a ref moved; only a
	// move reads the log again.
	gitLogPoll = 3 * time.Second
	gitLogPage = 200
)

// gitLogFilter is what narrows the history. It is comparable: a new value is a
// new read.
type gitLogFilter struct {
	ref, query, author, file string
	since                    int64
}

// narrowed is whether commits are left out of the page by something other than
// reachability. Their parents may then be missing, so no graph can be drawn.
func (f gitLogFilter) narrowed() bool {
	return f.query != "" || f.author != "" || f.file != "" || f.since != 0
}

// gitLogView is one frame's reading of the store.
type gitLogView struct {
	refs    backend.GitRefs
	commits []backend.GitCommit
	rows    []gitGraphRow
	lanes   int
	loaded  bool
	more    bool
	err     string
}

// gitLoaded is a single answer, kept until another key is asked for. While the
// next key loads, val is still the last answer and prev says it is there, so
// the panel keeps showing it rather than flashing a skeleton between two.
type gitLoaded[T any] struct {
	key    string
	val    T
	loaded bool
	prev   bool
	err    string
}

type gitDiffKey struct{ hash, file, oldFile string }

// gitStore reads the history of the path the panel shows, off the UI
// goroutine, while the panel is open.
type gitStore struct {
	ctx        context.Context
	client     *backend.Client
	invalidate func()

	mu      sync.Mutex
	path    string
	filter  gitLogFilter
	stop    context.CancelFunc
	kick    chan struct{}
	view    gitLogView
	reads   int // counts the reads started, so an older one's answer is dropped
	loading bool
	detail  gitLoaded[backend.GitCommitDetail]
	diff    gitLoaded[string]
	authors gitLoaded[[]string]
}

// follow points the store at path and filter, starting over when either
// changed.
func (s *gitStore) follow(path string, f gitLogFilter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil && s.path == path && s.filter == f {
		return
	}
	if s.stop != nil {
		s.stop()
	}
	ctx, stop := context.WithCancel(s.ctx)
	s.path, s.filter, s.stop, s.kick = path, f, stop, make(chan struct{}, 1)
	s.view, s.loading = gitLogView{}, false
	s.reads++
	go s.poll(ctx, path, f, s.kick)
}

func (s *gitStore) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil {
		s.stop()
		s.stop = nil
	}
}

func (s *gitStore) snapshot() gitLogView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view
}

// refresh asks for the refs now, after a change the panel made itself.
func (s *gitStore) refresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

func (s *gitStore) poll(ctx context.Context, path string, f gitLogFilter, kick chan struct{}) {
	tick := time.NewTicker(gitLogPoll)
	defer tick.Stop()
	stamp := ""
	for {
		refs, err := s.client.GitRefs(ctx, path)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			s.fail(ctx, err)
		case refs.Stamp != stamp:
			stamp = refs.Stamp
			s.reload(ctx, path, f, refs)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-kick:
		}
	}
}

func (s *gitStore) fail(ctx context.Context, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	s.mu.Lock()
	if ctx.Err() == nil {
		s.view.err, s.view.loaded = backend.ErrorText(err), true
	}
	s.mu.Unlock()
	s.invalidate()
}

// reload reads the history again, as many commits as are on screen so a ref
// moving does not throw away what was paged in.
func (s *gitStore) reload(ctx context.Context, path string, f gitLogFilter, refs backend.GitRefs) {
	s.mu.Lock()
	s.reads++
	read, limit := s.reads, max(gitLogPage, len(s.view.commits))
	s.mu.Unlock()
	page, err := s.client.GitLog(ctx, gitLogOptions(path, f, 0, limit))
	if err != nil {
		s.fail(ctx, err)
		return
	}
	s.mu.Lock()
	if read == s.reads && ctx.Err() == nil {
		s.view = buildGitLogView(refs, page.Commits, page.More, f)
		s.loading = false
	}
	s.mu.Unlock()
	s.invalidate()
}

// more reads the next page, if there is one and nothing is reading already.
func (s *gitStore) more() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loading || !s.view.more || s.stop == nil {
		return
	}
	s.loading = true
	s.reads++
	read, path, f, skip := s.reads, s.path, s.filter, len(s.view.commits)
	go func() {
		page, err := s.client.GitLog(s.ctx, gitLogOptions(path, f, skip, gitLogPage))
		s.mu.Lock()
		defer s.invalidate()
		defer s.mu.Unlock()
		if read != s.reads {
			return
		}
		s.loading = false
		if err != nil {
			log.Printf("git log page: %v", err)
			s.view.more = false
			return
		}
		s.view = buildGitLogView(s.view.refs, append(s.view.commits[:len(s.view.commits):len(s.view.commits)], page.Commits...), page.More, f)
	}()
}

func gitLogOptions(path string, f gitLogFilter, skip, limit int) backend.GitLogOptions {
	return backend.GitLogOptions{Path: path, Ref: f.ref, Query: f.query, Author: f.author, Since: f.since, File: f.file, Skip: skip, Limit: limit}
}

func buildGitLogView(refs backend.GitRefs, commits []backend.GitCommit, more bool, f gitLogFilter) gitLogView {
	v := gitLogView{refs: refs, commits: commits, more: more, loaded: true}
	if !f.narrowed() {
		v.rows, v.lanes = layoutGitGraph(commits)
	}
	return v
}

// gitFetch answers slot for key, starting the read the first time key is asked
// for. Callers must not hold s.mu.
func gitFetch[T any](s *gitStore, slot *gitLoaded[T], key string, load func(context.Context) (T, error)) gitLoaded[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slot.key == key {
		return *slot
	}
	*slot = gitLoaded[T]{key: key, val: slot.val, prev: slot.loaded || slot.prev}
	go func() {
		val, err := load(s.ctx)
		s.mu.Lock()
		defer s.invalidate()
		defer s.mu.Unlock()
		if slot.key != key {
			return
		}
		slot.val, slot.loaded = val, true
		if err != nil {
			slot.err = backend.ErrorText(err)
		}
	}()
	return *slot
}

func (s *gitStore) commit(path, hash string) gitLoaded[backend.GitCommitDetail] {
	return gitFetch(s, &s.detail, path+"\x00"+hash, func(ctx context.Context) (backend.GitCommitDetail, error) {
		return s.client.GitCommit(ctx, path, hash)
	})
}

// authorsOf is who wrote the whole history on screen, read again when a ref
// moves. The authors of the commits loaded stand in until that answers, and
// when it fails, as it does on a history too large to read in time.
func (s *gitStore) authorsOf(view gitLogView) []string {
	s.mu.Lock()
	path, ref := s.path, s.filter.ref
	s.mu.Unlock()
	if !view.loaded {
		return gitAuthors(view.commits)
	}
	read := gitFetch(s, &s.authors, path+"\x00"+ref+"\x00"+view.refs.Stamp, func(ctx context.Context) ([]string, error) {
		return s.client.GitAuthors(ctx, path, ref)
	})
	if read.err != "" || !(read.loaded || read.prev) {
		return gitAuthors(view.commits)
	}
	return gitTopAuthors(read.val)
}

func (s *gitStore) fileDiff(path string, k gitDiffKey) gitLoaded[string] {
	return gitFetch(s, &s.diff, path+"\x00"+k.hash+"\x00"+k.file+"\x00"+k.oldFile, func(ctx context.Context) (string, error) {
		return s.client.GitCommitDiff(ctx, path, k.hash, k.file, k.oldFile)
	})
}
