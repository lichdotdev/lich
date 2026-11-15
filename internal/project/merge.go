package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
)

// gitDirs is each checkout's git directory, asked of git once: a merge in
// progress is MERGE_HEAD in it, which git status never reports, and the poll
// reads it every few seconds per checkout on screen.
var gitDirs sync.Map // checkout path -> absolute git dir

func gitDir(path string) (string, bool) {
	if dir, ok := gitDirs.Load(path); ok {
		return dir.(string), true
	}
	out, ok := gitQuiet(path, "rev-parse", "--absolute-git-dir")
	if !ok {
		return "", false
	}
	dir := strings.TrimSpace(out)
	gitDirs.Store(path, dir)
	return dir, true
}

// mergingFrom names what the merge in progress at path brings in, "" when
// there is none.
func mergingFrom(path string) string {
	dir, ok := gitDir(path)
	if !ok {
		return ""
	}
	head, err := os.ReadFile(filepath.Join(dir, "MERGE_HEAD"))
	if err != nil {
		// A worktree removed and made again at the same path gets a new git dir.
		if _, gone := os.Stat(dir); gone != nil {
			gitDirs.Delete(path)
		}
		return ""
	}
	msg, _ := os.ReadFile(filepath.Join(dir, "MERGE_MSG"))
	return mergeSource(string(msg), string(head))
}

// mergeSource reads the merged name off the message git prepared ("Merge
// branch 'x'", "Merge remote-tracking branch 'origin/x'", "Merge tag 'v1'"),
// whose first quoted name is the first one merged. A merge asked for by full
// ref name spells it that way in the message, so the ref prefix goes. A
// message written with -m names nothing, and the commit's short hash stands in.
func mergeSource(msg, head string) string {
	first, _, _ := strings.Cut(msg, "\n")
	if rest, ok := strings.CutPrefix(first, "Merge "); ok {
		if _, quoted, ok := strings.Cut(rest, "'"); ok {
			if name, _, ok := strings.Cut(quoted, "'"); ok && name != "" {
				for _, prefix := range []string{"refs/heads/", "refs/remotes/", "refs/tags/"} {
					name = strings.TrimPrefix(name, prefix)
				}
				return name
			}
		}
	}
	hash, _, _ := strings.Cut(strings.TrimSpace(head), "\n")
	return hash[:min(len(hash), 7)]
}

var errNoMerge = errors.New("There is no merge in progress in this checkout.")

// GitMergeAbort puts the checkout back where it stood before the merge,
// dropping whatever was resolved since.
func (s *Service) GitMergeAbort(_ context.Context, opts api.PathOptions) (api.None, error) {
	if mergingFrom(opts.Path) == "" {
		return api.None{}, errNoMerge
	}
	_, err := runGit(opts.Path, "merge", "--abort")
	return api.None{}, err
}

// GitMergeCommit concludes the merge with the message git prepared. Refused
// outside a merge, where the same commit would take whatever happens to be
// staged.
func (s *Service) GitMergeCommit(_ context.Context, opts api.PathOptions) (api.None, error) {
	if mergingFrom(opts.Path) == "" {
		return api.None{}, errNoMerge
	}
	_, err := runGit(opts.Path, "commit", "--no-edit")
	return api.None{}, err
}

// GitMarkResolved stages a conflicted file as it stands, which is how git
// records it resolved.
func (s *Service) GitMarkResolved(_ context.Context, opts api.CheckoutFileOptions) (api.None, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return api.None{}, err
	}
	if !slices.Contains(readWorkTree(opts.Path).conflicted, opts.Rel) {
		return api.None{}, errors.New("That file has no conflict to resolve.")
	}
	_, err := runGit(opts.Path, "add", "--", opts.Rel)
	return api.None{}, err
}

// refuseMidMerge keeps a revert off a merge in progress: reverting a file to
// HEAD there drops what the merge brought, or a conflict, with nothing saying
// so. Abort is the way back.
func refuseMidMerge(path, rel string) error {
	if mergingFrom(path) != "" || slices.Contains(readWorkTree(path).conflicted, rel) {
		return errors.New("A merge is in progress here. Abort it to go back, or resolve the conflict.")
	}
	return nil
}

// commitOf resolves ref to the commit it names, refusing what git would read
// as an option.
func commitOf(path, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("%q is not a branch or commit to merge.", ref)
	}
	out, err := runGit(path, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return strings.TrimSpace(out), err
}

// GitMergePreview reads, without touching the checkout, what merging opts.Ref
// would bring. The conflicts come from merge-tree, so a git older than 2.38
// answers with an error rather than a guess.
func (s *Service) GitMergePreview(_ context.Context, opts api.GitMergeOptions) (api.GitMergePreview, error) {
	if _, err := commitOf(opts.Path, opts.Ref); err != nil {
		return api.GitMergePreview{}, err
	}
	out, err := readGit(opts.Path, "rev-list", "--count", "HEAD.."+opts.Ref)
	if err != nil {
		return api.GitMergePreview{}, err
	}
	commits, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return api.GitMergePreview{}, fmt.Errorf("read the commit count: %w", err)
	}
	if commits == 0 {
		return api.GitMergePreview{}, nil
	}
	conflicts, ok := mergeConflicts(opts.Path, "HEAD", opts.Ref)
	if !ok {
		return api.GitMergePreview{}, errors.New("Predicting the conflicts needs git 2.38 or newer.")
	}
	return api.GitMergePreview{Commits: commits, Conflicts: conflicts}, nil
}

// GitMerge merges opts.Ref into the checkout. It refuses a checkout with
// uncommitted changes, so Abort always goes back to where the merge started,
// and a merge already running. A merge that stops on conflicts is an
// outcome, not an error: it answers with the files.
func (s *Service) GitMerge(_ context.Context, opts api.GitMergeOptions) (api.GitMergeResult, error) {
	if _, err := commitOf(opts.Path, opts.Ref); err != nil {
		return api.GitMergeResult{}, err
	}
	status := readWorkTree(opts.Path)
	switch {
	case mergingFrom(opts.Path) != "":
		return api.GitMergeResult{}, errors.New("A merge is already in progress here.")
	case status.files > 0:
		return api.GitMergeResult{}, errors.New("This checkout has uncommitted changes. Commit or stash them before merging.")
	}
	result := api.GitMergeResult{Before: status.head}
	args := []string{"merge", "--no-edit", opts.Ref}
	cmd := command("git", append([]string{"-C", opts.Path}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Read before logging: a conflict is the merge working, not a failure.
		if conflicts := readWorkTree(opts.Path).conflicted; len(conflicts) > 0 {
			result.Conflicts = conflicts
			return result, nil
		}
		return api.GitMergeResult{}, gitFailure(args, stderr.String(), err)
	}
	result.After = readWorkTree(opts.Path).head
	return result, nil
}

// GitMergeUndo takes a merge back for as long as nothing has moved HEAD past
// it: reset --keep refuses to touch an edit made since.
func (s *Service) GitMergeUndo(_ context.Context, opts api.GitMergeUndoOptions) (api.None, error) {
	before, err := commitOf(opts.Path, opts.Before)
	if err != nil {
		return api.None{}, err
	}
	if head := readWorkTree(opts.Path).head; head == "" || head != opts.After {
		return api.None{}, errors.New("The branch has moved since the merge, so it cannot be undone from here.")
	}
	_, err = runGit(opts.Path, "reset", "--keep", before)
	return api.None{}, err
}
