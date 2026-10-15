package project

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// DiffText returns the full unified diff of uncommitted changes (staged +
// unstaged + untracked) against HEAD. A repository without commits diffs
// against git's empty tree. Untracked files are rendered as new-file hunks so
// the review panel shows them alongside tracked changes.
func (s *Service) DiffText(path string) (string, error) {
	status := readWorkTree(path)
	tracked, err := runGit(path, "diff", status.base)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	out.WriteString(tracked)
	for _, rel := range status.untracked {
		out.WriteString(untrackedDiff(path, rel))
	}
	return displayDiff(out.String()), nil
}

// displayDiff is displayText line by line: one diff carries every changed
// file, so a Latin-1 file's lines are decoded without touching a UTF-8 one's.
func displayDiff(diff string) string {
	if utf8.ValidString(diff) {
		return diff
	}
	lines := strings.SplitAfter(diff, "\n")
	for i, line := range lines {
		lines[i] = displayText([]byte(line))
	}
	return strings.Join(lines, "")
}

// untrackedDiff renders an untracked file as a new-file unified diff via
// git diff --no-index, which exits 1 when the files differ — success here.
// Any other failure (file vanished since the status read) yields "".
func untrackedDiff(dir, rel string) string {
	if info, err := os.Stat(filepath.Join(dir, rel)); err != nil || !info.Mode().IsRegular() || info.Size() > maxTextFileBytes {
		return ""
	}
	cmd := command("git", "-C", dir, "diff", "--no-index", "--", os.DevNull, rel)
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if err != nil && (!errors.As(err, &exitErr) || exitErr.ExitCode() != 1) {
		return ""
	}
	return string(out)
}
