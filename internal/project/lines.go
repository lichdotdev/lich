package project

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
)

// FileLines is how far one file stands from the diff base, as `git diff
// --numstat` counts it; a binary file is Binary with no counts.
type FileLines struct {
	Added, Removed int
	Binary         bool
}

// Lines is a checkout against its diff base, file by file, with the commit it
// was read on: a reading is only comparable to another of the same head.
type Lines struct {
	Head  string
	Files map[string]FileLines
}

// ReadLines reads path's work tree against HEAD, a new untracked file counted
// as all added the way Diff counts it. It reports false where git could not
// answer: not a repository, or ctx ran out.
func ReadLines(ctx context.Context, path string) (Lines, bool) {
	status := readWorkTreeContext(ctx, path)
	lines := Lines{Head: status.head, Files: map[string]FileLines{}}
	// A clean tree has nothing to count, and a repository the status read
	// could not see is told apart by the diff failing below.
	if status.files > 0 || status.head == "" {
		out, ok := gitQuietContext(ctx, path, "diff", "--numstat", "-z", "--relative", status.base)
		if !ok {
			return Lines{}, false
		}
		parseNumstat(out, lines.Files)
	}
	for _, rel := range status.untracked {
		lines.Files[rel] = FileLines{Added: countFileLines(filepath.Join(path, rel))}
	}
	return lines, ctx.Err() == nil
}

// parseNumstat reads `git diff --numstat -z` into files. A rename spells its
// two paths as records of their own after an empty one; the new path is kept.
func parseNumstat(out string, files map[string]FileLines) {
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		cols := strings.SplitN(records[i], "\t", 3)
		if len(cols) < 3 {
			continue
		}
		path := cols[2]
		if path == "" {
			if i+2 >= len(records) {
				return
			}
			path = records[i+2]
			i += 2
		}
		added, errA := strconv.Atoi(cols[0])
		removed, errR := strconv.Atoi(cols[1])
		files[path] = FileLines{Added: added, Removed: removed, Binary: errA != nil || errR != nil}
	}
}
