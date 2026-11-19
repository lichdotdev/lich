package chat

import (
	"context"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/project"
)

// linesTimeout bounds one reading of the checkout at a turn's edge: a turn
// begins and ends behind it, so a slow repository costs the turn no more.
const linesTimeout = 5 * time.Second

// readLines is the session's checkout against HEAD, nil where git could not
// say: the turn then ends without a count rather than with a wrong one.
func (c *chatSession) readLines() *project.Lines {
	ctx, cancel := context.WithTimeout(context.Background(), linesTimeout)
	defer cancel()
	lines, ok := project.ReadLines(ctx, c.cwd)
	if !ok {
		return nil
	}
	return &lines
}

// turnChanges is what a turn changed: the files whose count differs from the
// turn's start and the lines each gained or lost. nil where either reading is
// missing or the checkout moved to another commit, which a diff against HEAD
// cannot compare.
func turnChanges(before, after *project.Lines) *api.ChatTurnChanges {
	if before == nil || after == nil || before.Head != after.Head {
		return nil
	}
	changes := &api.ChatTurnChanges{}
	count := func(name string, was, now project.FileLines) {
		if was == now {
			return
		}
		changes.Files++
		changes.Added += max(now.Added-was.Added, 0)
		changes.Removed += max(now.Removed-was.Removed, 0)
	}
	for name, now := range after.Files {
		count(name, before.Files[name], now)
	}
	for name, was := range before.Files {
		if _, kept := after.Files[name]; !kept {
			count(name, was, project.FileLines{})
		}
	}
	return changes
}
