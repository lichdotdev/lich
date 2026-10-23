package project

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
)

// DiscardFile reverts one file's uncommitted changes. A file known to HEAD is
// checked out from it (restoring index and worktree in one step); a new file
// (staged or untracked) is unstaged and deleted from disk. opts.Rel must be a
// repo-relative path — the review panel passes paths parsed from git's own
// diff output.
func (s *Service) DiscardFile(_ context.Context, opts api.CheckoutFileOptions) (api.None, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return api.None{}, err
	}
	// Quiet: "HEAD does not know this file" is the answer that routes to the
	// new-file branch below, not a failure worth a log line per discard.
	if _, known := gitQuiet(opts.Path, "cat-file", "-e", "HEAD:"+opts.Rel); known {
		_, err := runGit(opts.Path, "checkout", "HEAD", "--", opts.Rel)
		return api.None{}, err
	}
	if _, err := runGit(opts.Path, "rm", "-f", "--cached", "--ignore-unmatch", "--", opts.Rel); err != nil {
		return api.None{}, err
	}
	if err := os.Remove(filepath.Join(opts.Path, opts.Rel)); err != nil && !os.IsNotExist(err) {
		return api.None{}, fmt.Errorf("remove %s: %w", opts.Rel, err)
	}
	return api.None{}, nil
}
