package project

import (
	"context"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// emptyTreeHash is git's well-known empty tree object, the diff base for a
// repository whose HEAD does not exist yet (no commits).
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// workTreeStatus is everything one `git status --porcelain=v2 --branch` read
// answers at once: the branch, the commit it sits on, and every dirty entry.
// It replaces three separate git children — `symbolic-ref --short HEAD`,
// `rev-parse --verify HEAD` and `ls-files --others` — on a read the window
// polls every few seconds per checkout on screen, where process startup, not work, is
// what a tick costs on a normal repository.
type workTreeStatus struct {
	branch    string   // "" for a detached HEAD, as Branch answers
	head      string   // "" in a repository with no commits
	base      string   // what a diff runs against: "HEAD", or the empty tree
	files     int      // dirty entries: changed, renamed, unmerged, untracked
	untracked []string // untracked paths, relative to the path that was read
	// conflicted is the unmerged paths, relative to the repository root as git
	// diff names them, which is how the review panel matches them.
	conflicted []string
	// changes is every dirty file still on disk by its state, relative to the
	// path that was read.
	changes map[string]string
}

// readWorkTree runs the single status call and parses it. A path git will not
// read as a work tree yields the zero readout, whose base is the empty tree —
// the same pair diffBase handed back before, so the callers behind it fail the
// way they always did rather than on a new code path.
//
// -z, because the counters index the untracked paths straight into the
// filesystem and v2 C-quotes them otherwise. --untracked-files=all, because the
// default collapses a new directory into one "? dir/" entry: an agent writing
// 25 files into fresh packages would count as one changed file.
func readWorkTree(path string) workTreeStatus {
	return readWorkTreeContext(context.Background(), path)
}

// readWorkTreeContext is readWorkTree for a read that must not outlive ctx.
func readWorkTreeContext(ctx context.Context, path string) workTreeStatus {
	out, ok := gitQuietContext(ctx, path, "status", "--porcelain=v2", "--branch",
		"--untracked-files=all", "-z")
	if !ok {
		return workTreeStatus{base: emptyTreeHash}
	}
	status := parseWorkTree(out)
	// Asked for only when there is something to translate: on a clean tree the
	// prefix would cost a second git child to be told nothing, on the read the
	// window polls every few seconds per checkout on screen.
	if len(status.changes) > 0 {
		status.untracked, status.changes = relativeToPath(ctx, path, status.untracked, status.changes)
	}
	return status
}

// relativeToPath translates the untracked and changed paths porcelain v2 reports — always
// relative to the repository root, and covering the whole repository, which
// --porcelain forces whatever status.relativePaths says — back to being
// relative to path, the way the `ls-files --others` call this read replaced
// reported them. Both callers join them onto path to reach the file.
//
// git answers where path sits with `rev-parse --show-prefix`: "sub/" from a
// subdirectory, "" when path is the repository root itself. Entries outside the
// prefix are dropped rather than kept: they live outside the directory the
// caller asked about, and `ls-files --others`, scoped to it, never reported
// them either.
func relativeToPath(ctx context.Context, path string, untracked []string, changes map[string]string) ([]string, map[string]string) {
	out, ok := gitQuietContext(ctx, path, "rev-parse", "--show-prefix")
	// The overwhelmingly common case: path is the repository root, and every
	// entry is already relative to it. A git that refused the question leaves
	// them alone too — the same paths the read handed back before this call.
	prefix := strings.TrimRight(out, "\r\n")
	if !ok || prefix == "" {
		return untracked, changes
	}
	scoped := untracked[:0]
	for _, rel := range untracked {
		if under, found := strings.CutPrefix(rel, prefix); found {
			scoped = append(scoped, under)
		}
	}
	under := make(map[string]string, len(changes))
	for rel, change := range changes {
		if inside, found := strings.CutPrefix(rel, prefix); found {
			under[inside] = change
		}
	}
	return scoped, under
}

// parseWorkTree reads porcelain v2's NUL-terminated records. Every record shape
// git does not promise here is skipped rather than guessed at: this one parse
// feeds the branch, the head and the dirty count together, so a line it cannot
// read must cost that line alone.
func parseWorkTree(out string) workTreeStatus {
	status := workTreeStatus{base: emptyTreeHash}
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		switch record[0] {
		case '#':
			status.readHeader(record)
		case '2':
			// A rename or copy spells its original path as a second record of
			// its own, which v1 kept on the one "R orig -> new" line. Consumed
			// here, or one renamed file would count as two dirty ones.
			i++
			status.files++
			// 2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <Xscore> <path>
			if fields := strings.SplitN(record, " ", 10); len(fields) == 10 {
				status.change(fields[9], api.FileRenamed, fields[1])
			}
		case '1':
			status.files++
			// 1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
			if fields := strings.SplitN(record, " ", 9); len(fields) == 9 {
				status.change(fields[8], changeOf(fields[1]), fields[1])
			}
		case 'u':
			status.files++
			// u <XY> <sub> <m1> <m2> <m3> <mW> <h1> <h2> <h3> <path>
			if fields := strings.SplitN(record, " ", 11); len(fields) == 11 {
				status.conflicted = append(status.conflicted, fields[10])
				status.change(fields[10], api.FileConflict, "")
			}
		case '?':
			status.files++
			if _, path, ok := strings.Cut(record, " "); ok {
				status.untracked = append(status.untracked, path)
				status.change(path, api.FileUntracked, "")
			}
		}
	}
	return status
}

// changeOf is an ordinary entry's state from its XY: staged as new reads as
// added, anything else as modified.
func changeOf(xy string) string {
	if strings.HasPrefix(xy, "A") {
		return api.FileAdded
	}
	return api.FileModified
}

// change notes path's state, unless xy says the file is gone from disk: the
// Files tree lists only what is there.
func (s *workTreeStatus) change(path string, state string, xy string) {
	if strings.Contains(xy, "D") {
		return
	}
	if s.changes == nil {
		s.changes = map[string]string{}
	}
	s.changes[path] = state
}

// readHeader takes the two `# branch.*` lines the readout needs, mapping v2's
// sentinels back onto the contracts the separate calls held: a detached HEAD
// names no branch, and a repository with no commits has no HEAD to diff
// against — git's empty tree stands in for it. Left unmapped, the badge would
// read a literal "(detached)" and a diff would run against "(initial)".
func (s *workTreeStatus) readHeader(record string) {
	key, value, ok := strings.Cut(strings.TrimPrefix(record, "# "), " ")
	if !ok {
		return
	}
	switch key {
	case "branch.head":
		if value != "(detached)" {
			s.branch = value
		}
	case "branch.oid":
		if value != "(initial)" {
			s.head, s.base = value, "HEAD"
		}
	}
}
