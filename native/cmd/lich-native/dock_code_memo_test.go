package main

import (
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// codeMemoBackend answers the Code tab's reads of a checkout whose files a
// test changes, and records the searches it was asked for.
type codeMemoBackend struct {
	mu       sync.Mutex
	files    []string
	searches []string
}

func (b *codeMemoBackend) set(files ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.files = files
}

func (b *codeMemoBackend) searched() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.searches)
}

// codeMemoFrames is the Code tab of a git checkout, whose tree is read again
// only when its stats move: a frame laid out again is one the tab asked for.
func codeMemoFrames(t *testing.T, files ...string) (*codeTab, *frameRig, *codeMemoBackend, *dockInput) {
	t.Helper()
	b := &codeMemoBackend{files: files}
	in := &dockInput{checkout: "/repo", isGit: true, git: backend.DiffStats{Files: 1}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		var answer any
		switch strings.TrimPrefix(r.URL.Path, "/rpc/") {
		case "project.Tree":
			answer = fileListing{Files: b.files}
		case "project.Search":
			var args []api.SearchOptions
			_ = json.NewDecoder(r.Body).Decode(&args)
			if len(args) > 0 {
				b.searches = append(b.searches, args[0].Query)
			}
			answer = searchResult{Matches: []searchMatch{{Path: "a.go", Line: 2, Text: "needle"}}}
		}
		_ = json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(srv.Close)
	r := newFrameRig(image.Pt(500, 600), nil)
	c := newCodeTab(clientOf(srv), r.invalidate, new(commentStore), func(ui.Toast) {}, func(string, string, int) {})
	root := new(ui.Root)
	r.draw = func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			c.layout(gtx, th, *in)
			return D{Size: gtx.Constraints.Max}
		})
	}
	return c, r, b, in
}

// The tab replays while nothing changes; a re-read listing a new file still
// reaches the screen.
func TestCodeTreeShowsAFileThatArrivesWhileTheTabReplays(t *testing.T) {
	_, r, b, in := codeMemoFrames(t, "a.go")
	r.waitFor(t, "a.go")
	r.frames(3)
	b.set("a.go", "b.go")
	// The stats moving reads the tree again; the listing lands frames later.
	in.git.Files = 2
	r.frame()
	r.waitFor(t, "b.go")
}

func TestCodeTreeFolderTakesAClickAfterReplayedFrames(t *testing.T) {
	c, r, _, _ := codeMemoFrames(t, "pkg/x.go")
	r.waitFor(t, "pkg")
	r.frames(3)
	open := c.toggled["pkg"]
	r.clickLabel(t, "pkg")
	if c.toggled["pkg"] == open {
		t.Fatal("a click on a replayed folder did not fold or unfold it")
	}
}

// A hovered file lights up: the hover reaches a row the tab replayed.
func TestCodeTreeRowLightsUpOnAHoverAfterReplayedFrames(t *testing.T) {
	_, r, _, _ := codeMemoFrames(t, "a.go", "b.go")
	r.waitFor(t, "a.go")
	r.frames(3)
	before, _ := r.frame()
	r.hover(r.at(t, "a.go"))
	after, _ := r.frame()
	if paintOf(before) == paintOf(after) {
		t.Fatal("hovering a replayed file row drew nothing new")
	}
}

// The content search waits for typing to pause: the frame it asked for
// comes while the tab replays, and the search goes out.
func TestCodeSearchGoesOutOnceTypingPausesWhileTheTabReplays(t *testing.T) {
	c, r, b, _ := codeMemoFrames(t, "a.go")
	r.waitFor(t, "a.go")
	c.openSearch()
	c.query.SetText("needle")
	r.frame()
	for end := r.now.Add(time.Second); r.now.Before(end) && !slices.Contains(b.searched(), "needle"); {
		r.now = r.now.Add(frameStep)
		r.frame()
		time.Sleep(time.Millisecond)
	}
	if !slices.Contains(b.searched(), "needle") {
		t.Fatalf("the content search never went out after typing paused; searches: %q", b.searched())
	}
	r.waitFor(t, "needle")
}
