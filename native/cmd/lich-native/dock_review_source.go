package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/omartelo/lich/native/lichclient"
)

// ponytail: English until the native window has i18n (diff.reviewPanel).
const (
	reviewNotRepository = "Not a git repository"
	reviewLoading       = "Loading…"
	reviewNoChanges     = "No uncommitted changes"
	reviewSqueezed      = "Side-by-side needs a wider panel"
)

// diffPoll is ReviewPanel's DIFF_POLL_MS: how often the diff is read again,
// catching the edits the status counts are blind to.
const diffPoll = 2 * time.Second

// diffRead is one read of the checkout's working-tree diff.
type diffRead struct {
	text   string
	failed bool
}

// diffFeed reads a checkout's diff off the UI goroutine, again every
// diffPoll and at once on refresh, and keeps the newest read for the frame.
type diffFeed struct {
	client     *lichclient.Client
	invalidate func()

	mu   sync.Mutex
	path string
	stop context.CancelFunc
	read *diffRead
	kick chan struct{}
}

// follow points the feed at path, starting over when it changed.
func (f *diffFeed) follow(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.path == path && f.stop != nil {
		return
	}
	if f.stop != nil {
		f.stop()
	}
	ctx, stop := context.WithCancel(context.Background())
	f.path, f.stop, f.read, f.kick = path, stop, nil, make(chan struct{}, 1)
	go f.poll(ctx, path, f.kick)
}

func (f *diffFeed) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stop != nil {
		f.stop()
		f.stop = nil
	}
}

// refresh reads again now, after an edit the panel made itself.
func (f *diffFeed) refresh() {
	f.mu.Lock()
	defer f.mu.Unlock()
	select {
	case f.kick <- struct{}{}:
	default:
	}
}

// touched reads again now when path is the checkout being followed: a
// session reported editing files in it.
func (f *diffFeed) touched(path string) {
	f.mu.Lock()
	follows := f.path == path
	f.mu.Unlock()
	if follows {
		f.refresh()
	}
}

// latest is the newest read, nil before the first lands.
func (f *diffFeed) latest() *diffRead {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.read
}

func (f *diffFeed) poll(ctx context.Context, path string, kick chan struct{}) {
	tick := time.NewTicker(diffPoll)
	defer tick.Stop()
	for {
		text, err := f.client.DiffText(ctx, path)
		if ctx.Err() != nil {
			return
		}
		r := diffRead{text: text}
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("review diff: %v", err)
			}
			r = diffRead{failed: true}
		}
		f.mu.Lock()
		f.read = &r
		f.mu.Unlock()
		f.invalidate()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-kick:
		}
	}
}
