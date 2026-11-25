package chat

import (
	"context"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/project"
)

// fileWatch tells which files each call of a turn changed: a shell command or
// a tool that says nothing of the files it writes (sed, a script, an MCP tool)
// shows its edits as an edit of its own does. Every call that ends reads the
// checkout once and takes the files changed since the reading before, the
// turn's start for the first. The reading is git's per-file counts, the same
// the turn's end compares (changes.go).
type fileWatch struct {
	mu   sync.Mutex
	base *project.Lines
	// queued closes once the last call queued has had its reading.
	queued chan struct{}
}

// queue holds a call's place in line: its reading waits for the call that
// ended before it, so a file goes to the call that changed it even when the
// readings run off the stream in their own goroutines.
func (w *fileWatch) queue() (wait <-chan struct{}, done chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	done = make(chan struct{})
	wait, w.queued = w.queued, done
	return wait, done
}

// start sets the reading the turn's first call compares against.
func (w *fileWatch) start(base *project.Lines) {
	w.mu.Lock()
	w.base = base
	w.mu.Unlock()
}

// ended reads the checkout after a call ended and answers the files it
// changed with their diffs, none for a call that shows its own diff: its
// files are still taken, so the next call is not handed them. Calls end one
// reading at a time, so two that ran side by side do not both take a file.
func (w *fileWatch) ended(ctx context.Context, cwd string, ownDiff bool, read func() *project.Lines) []api.ChatFileChange {
	w.mu.Lock()
	defer w.mu.Unlock()
	after := read()
	names := changedFiles(w.base, after)
	w.base = after
	if ownDiff || len(names) == 0 {
		return nil
	}
	files := make([]api.ChatFileChange, 0, len(names))
	for _, name := range names {
		if diff := project.FileDiff(ctx, cwd, *after, name); diff != "" {
			files = append(files, api.ChatFileChange{Path: name, Diff: diff})
		}
	}
	return files
}

// changedFiles is the files whose counts differ between two readings of one
// commit, sorted; none where either is missing or the commit moved.
func changedFiles(before, after *project.Lines) []string {
	if before == nil || after == nil || before.Head != after.Head {
		return nil
	}
	var names []string
	for name, now := range after.Files {
		if before.Files[name] != now {
			names = append(names, name)
		}
	}
	for name := range before.Files {
		if _, kept := after.Files[name]; !kept {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// callEnded hands the files a call changed to its item, once git has said:
// the reading runs off the provider's stream, which goes on meanwhile.
func (c *chatSession) callEnded(it api.ChatItem) {
	own := it.Tool != nil && it.Tool.Diff != ""
	wait, done := c.files.queue()
	go func() {
		defer close(done)
		if wait != nil {
			<-wait
		}
		ctx, cancel := context.WithTimeout(context.Background(), linesTimeout)
		defer cancel()
		files := c.files.ended(ctx, c.cwd, own, c.readLines)
		if len(files) > 0 {
			c.conv.attachFiles(it.ID, files)
		}
	}()
}
