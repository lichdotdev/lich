package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	"github.com/lichdotdev/lich/native/backend"
)

// viewedTicks are a pull request's Viewed ticks by path, read from the store
// once its Files tab first opens and written through on every tick.
type viewedTicks struct {
	loaded bool
	files  map[string]backend.ViewedFile
}

// viewedState is a file's tick weighed against the diff it has now.
type viewedState int

const (
	notViewed viewedState = iota
	viewedAsIs
	changedSinceReview
)

// diffHash is what a tick keeps of a file's diff.
func diffHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (t *viewedTicks) state(path, hash string) viewedState {
	var f backend.ViewedFile
	ok := false
	if t != nil {
		f, ok = t.files[path]
	}
	switch {
	case !ok:
		return notViewed
	case f.Hash == hash:
		return viewedAsIs
	}
	return changedSinceReview
}

// changedSince counts the files of order whose diff moved since their tick,
// and when the pull request was last ticked.
func (t *viewedTicks) changedSince(order []string, hashOf func(path string) string) (int, time.Time) {
	n := 0
	for _, path := range order {
		if t.state(path, hashOf(path)) == changedSinceReview {
			n++
		}
	}
	var last int64
	for _, f := range t.files {
		last = max(last, f.ViewedAt)
	}
	return n, time.Unix(last, 0)
}

// reviewDay is the day of a review, the year only when it is not this one.
func reviewDay(at, now time.Time) string {
	at = at.In(now.Location())
	if at.Year() != now.Year() {
		return at.Format("Jan 2, 2006")
	}
	return at.Format("Jan 2")
}

// loadViewed reads the pull request's ticks off the frame. A read that fails
// leaves the tab with none, as before they were kept.
func (v *pullView) loadViewed(url string, ticks *viewedTicks) {
	client, invalidate := v.files.client, v.files.invalidate
	go func() {
		files, err := client.ViewedFiles(context.Background(), url)
		if err != nil {
			log.Printf("viewed files of %s: %v", url, err)
		}
		v.onFrame(func() {
			ticks.files, ticks.loaded = map[string]backend.ViewedFile{}, true
			for _, f := range files {
				ticks.files[f.Path] = f
			}
		})
		invalidate()
	}()
}

// markViewed ticks c and folds it.
func (p *reviewPanel) markViewed(c *fileCard) {
	p.pr.viewed.files[c.git.path] = backend.ViewedFile{Path: c.git.path, Hash: c.gitHash, ViewedAt: time.Now().Unix()}
	p.fold(c)
}

// saveViewed and saveUnviewed carry a tick the user made to the store.
func (p *reviewPanel) saveViewed(c *fileCard) {
	client, url, path, hash := p.client, p.pr.url, c.git.path, c.gitHash
	go func() {
		if err := client.MarkViewed(context.Background(), url, path, hash); err != nil {
			log.Printf("viewed %s: %v", path, err)
		}
	}()
}

func (p *reviewPanel) saveUnviewed(c *fileCard) {
	client, url, path := p.client, p.pr.url, c.git.path
	go func() {
		if err := client.UnmarkViewed(context.Background(), url, path); err != nil {
			log.Printf("unviewed %s: %v", path, err)
		}
	}()
}
