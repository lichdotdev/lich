package project

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
)

// ErrLinesMoved is what a revert answers when the lines it was asked about are
// no longer the lines on disk: the diff on screen is older than the file.
var ErrLinesMoved = errors.New("the file changed since this diff was drawn. Refresh and try again")

// ErrStagedDiverged is a revert refused because the index holds a third version
// of these lines, neither HEAD's nor the working tree's.
var ErrStagedDiverged = errors.New("these lines have staged changes that differ from the file. Discard the file instead")

// The two sides a RevertLine can name.
const (
	sideOld = "old"
	sideNew = "new"
)

type RevertLine = api.RevertLine

type RevertResult = api.RevertResult

// RevertLines puts the chosen changed lines of opts.Rel back to HEAD, leaving every
// other change in the file alone. The patch is rebuilt here from git's own diff
// rather than taken from the caller, so a stale screen fails instead of
// reverting whatever those line numbers point at now.
//
// The index is reverted with the working tree whenever it holds the change:
// the panel draws staged and unstaged as one diff, so reverting only the file
// would leave the line in the next commit with nothing on screen showing it.
func (s *Service) RevertLines(_ context.Context, opts api.RevertLinesOptions) (RevertResult, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return RevertResult{}, err
	}
	if len(opts.Lines) == 0 {
		return RevertResult{}, errors.New("no changed lines to revert")
	}
	base := readWorkTree(opts.Path).base
	tracked, err := isTracked(opts.Path, opts.Rel)
	if err != nil {
		return RevertResult{}, err
	}
	diff, err := fileDiff(opts.Path, opts.Rel, base, tracked)
	if err != nil {
		return RevertResult{}, err
	}
	hunks, err := parseHunks(diff)
	if err != nil {
		return RevertResult{}, err
	}
	patch, err := partialPatch(opts.Rel, hunks, opts.Lines)
	if err != nil {
		return RevertResult{}, err
	}
	if !applies(opts.Path, patch, "-R", "--check") {
		return RevertResult{}, ErrLinesMoved
	}
	indexPatch, err := stagedPatch(opts.Path, opts.Rel, base, opts.Lines, tracked)
	if err != nil {
		return RevertResult{}, err
	}
	result := RevertResult{Patch: patch, IndexPatch: indexPatch}
	return result, applyBoth(opts.Path, result, true)
}

// RestoreLines undoes RevertLines: the same patches applied forwards, to the
// same targets. They come back from the caller, so each is held to the one file
// it was built for before git sees it.
func (s *Service) RestoreLines(_ context.Context, opts api.RestoreLinesOptions) (api.None, error) {
	if err := relpath.Validate(opts.Rel); err != nil {
		return api.None{}, err
	}
	if err := checkPatchFile(opts.Undo.Patch, opts.Rel); err != nil {
		return api.None{}, err
	}
	if opts.Undo.IndexPatch != "" {
		if err := checkPatchFile(opts.Undo.IndexPatch, opts.Rel); err != nil {
			return api.None{}, err
		}
	}
	if !applies(opts.Path, opts.Undo.Patch, "--check") ||
		(opts.Undo.IndexPatch != "" && !applies(opts.Path, opts.Undo.IndexPatch, "--cached", "--check")) {
		return api.None{}, ErrLinesMoved
	}
	return api.None{}, applyBoth(opts.Path, opts.Undo, false)
}

// isTracked asks the index about rel. An empty answer is untracked; a git that
// fails to answer is an error, since reading it as untracked would diff the
// whole file as additions.
func isTracked(path, rel string) (bool, error) {
	out, err := runGit(path, "ls-files", "-z", "--", rel)
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// fileDiff is the one file's slice of what DiffText draws: tracked against the
// same base, or an untracked file as a new-file diff. Which one is decided by
// the index, never by the tracked diff coming back empty: a committed file with
// nothing left to diff would otherwise read as all additions, and a stale screen
// could revert lines straight out of a clean file.
func fileDiff(path, rel, base string, tracked bool) (string, error) {
	if !tracked {
		return untrackedDiff(path, rel), nil
	}
	return readGit(path, "diff", base, "--", rel)
}

// stagedPatch finds the picked lines in the index. Each is looked up in the
// HEAD→index diff, an addition first carried to its index line number through
// the index→worktree diff. None staged leaves the index alone; all staged
// reverts it too; some staged is a third version of these lines, refused.
//
// A git that cannot answer is an error, never "nothing staged": leaving the
// index behind unannounced is the trap this whole check exists to close.
func stagedPatch(path, rel, base string, lines []RevertLine, tracked bool) (string, error) {
	if !tracked {
		return "", nil
	}
	staged, err := hunksOf(path, "diff", "--cached", base, "--", rel)
	if err != nil {
		return "", err
	}
	unstaged, err := hunksOf(path, "diff", "--", rel)
	if err != nil {
		return "", err
	}
	var picks []RevertLine
	for _, line := range lines {
		if pick, ok := inIndex(staged, unstaged, line); ok {
			picks = append(picks, pick)
		}
	}
	if len(picks) == 0 {
		return "", nil
	}
	if len(picks) != len(lines) {
		return "", ErrStagedDiverged
	}
	patch, err := partialPatch(rel, staged, picks)
	if err != nil || !applies(path, patch, "-R", "--cached", "--check") {
		return "", ErrStagedDiverged
	}
	return patch, nil
}

func hunksOf(path string, args ...string) ([]patchHunk, error) {
	out, err := readGit(path, args...)
	if err != nil {
		return nil, err
	}
	return parseHunks(out)
}

// inIndex restates one pick in the index's numbering, if the index holds it.
func inIndex(staged, unstaged []patchHunk, line RevertLine) (RevertLine, bool) {
	number := line.Line
	if line.Side == sideNew {
		indexLine, unchanged := oldLineOf(unstaged, line.Line)
		if !unchanged {
			return RevertLine{}, false
		}
		number = indexLine
	}
	pick := RevertLine{Side: line.Side, Line: number, Text: line.Text}
	return pick, hasChange(staged, pickKey{pick.Side, pick.Line}, pick.Text)
}

// applyBoth applies the patches, the working tree's first. Both were checked
// already; should the index still refuse, the working tree is put back so a
// failure never leaves the two half-reverted.
func applyBoth(path string, result RevertResult, reverse bool) error {
	if err := applyPatch(path, result.Patch, direction(reverse)...); err != nil {
		return err
	}
	if result.IndexPatch == "" {
		return nil
	}
	if err := applyPatch(path, result.IndexPatch, append(direction(reverse), "--cached")...); err != nil {
		if rollback := applyPatch(path, result.Patch, direction(!reverse)...); rollback != nil {
			return fmt.Errorf("%w (and restoring the file failed: %v)", err, rollback)
		}
		return err
	}
	return nil
}

func direction(reverse bool) []string {
	if reverse {
		return []string{"-R"}
	}
	return nil
}

// applies reports whether git apply accepts the patch with these flags; a
// refusal is the answer, not a failure worth logging.
func applies(dir, patch string, args ...string) bool {
	_, err := gitApply(dir, patch, args)
	return err == nil
}

func applyPatch(dir, patch string, args ...string) error {
	stderr, err := gitApply(dir, patch, args)
	if err != nil {
		return gitFailure(append([]string{"apply"}, args...), stderr, err)
	}
	return nil
}

func gitApply(dir, patch string, args []string) (string, error) {
	full := append([]string{"-C", dir, "apply"}, args...)
	cmd := command("git", append(full, "-")...)
	cmd.Stdin = strings.NewReader(patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}
