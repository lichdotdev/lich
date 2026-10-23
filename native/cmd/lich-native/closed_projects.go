package main

import (
	"context"
	"log"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
)

// closedProjects is the projects closed earlier, as the window knows them: one
// answer per search term, read by the open-project menu (the plain list) and by
// the command palette (whatever it searches). Entries are fetched on the first
// read. The backend says when the list changed (project-closed, project-added,
// project-opened) and the model marks everything stale; the next read of each
// term fetches again, and until it lands the held answer stays on screen.
//
// Only the plain list and the palette's current term are kept: the palette
// types its way through terms and an answer for each would only pile up.
type closedProjects struct {
	client     *backend.Client
	invalidate func()

	mu      sync.Mutex
	gen     int
	entries map[string]*closedEntry
}

type closedEntry struct {
	// gen is the generation the answer was asked for. A failed ask settles it
	// too, so a backend that is refusing is asked once per change, not once
	// per frame.
	gen      int
	tried    bool
	fetching bool
	answer   closedAnswer
}

func newClosedProjects(c *backend.Client, invalidate func()) *closedProjects {
	return &closedProjects{client: c, invalidate: invalidate, entries: map[string]*closedEntry{}}
}

// stale marks every held answer out of date.
func (b *closedProjects) stale() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gen++
}

// read answers what is held for term, empty until the first fetch lands, and
// starts a fetch when the answer is not for the current generation.
func (b *closedProjects) read(term string) closedAnswer {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.entries[term]
	if e == nil {
		if term != "" {
			for held := range b.entries {
				if held != "" {
					delete(b.entries, held)
				}
			}
		}
		e = &closedEntry{}
		b.entries[term] = e
	}
	if !e.fetching && (!e.tried || e.gen != b.gen) {
		e.fetching, e.tried, e.gen = true, true, b.gen
		go b.fetch(term, e)
	}
	return e.answer
}

func (b *closedProjects) fetch(term string, e *closedEntry) {
	answer, err := fetchClosed(context.Background(), b.client, term)
	if err != nil {
		log.Printf("closed projects: %v", err)
	}
	b.mu.Lock()
	e.fetching = false
	if err == nil {
		e.answer = answer
	}
	b.mu.Unlock()
	b.invalidate()
}

func fetchClosed(ctx context.Context, c *backend.Client, term string) (closedAnswer, error) {
	rows, err := c.RecentProjectsMatching(ctx, term)
	if err != nil {
		return closedAnswer{}, err
	}
	a := closedAnswer{rows: rows, total: len(rows), missing: map[string]bool{}}
	// What the count and the filesystem say is what a row says about itself,
	// and a failed check must not cost the palette its entries.
	if n, err := c.ClosedProjectCountMatching(ctx, term); err != nil {
		log.Printf("count closed projects: %v", err)
	} else {
		a.total = n
	}
	paths := make([]string, len(rows))
	for i, r := range rows {
		paths[i] = r.Path
	}
	gone, err := c.MissingPaths(ctx, paths)
	if err != nil {
		log.Printf("check closed project paths: %v", err)
	}
	for _, path := range gone {
		a.missing[path] = true
	}
	return a, nil
}
