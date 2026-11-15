package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
// whose first quoted name is the first one merged. A message written with -m
// names nothing, and the commit's short hash stands in.
func mergeSource(msg, head string) string {
	first, _, _ := strings.Cut(msg, "\n")
	if rest, ok := strings.CutPrefix(first, "Merge "); ok {
		if _, quoted, ok := strings.Cut(rest, "'"); ok {
			if name, _, ok := strings.Cut(quoted, "'"); ok && name != "" {
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
