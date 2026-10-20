package project

import (
	"os"
	"path/filepath"
	"strings"
)

// SetupScriptPath is the repo-relative path of the script a new worktree's
// terminal runs once before its provider starts. It lives in the repository
// rather than lich's store so it is versioned and shared like the code it
// bootstraps; an untracked copy works too. Exported because a session that
// skips it names the file on screen (internal/terminal.setupSkippedNotice).
const SetupScriptPath = ".lich/setup-worktree.sh"

// SetupScript returns the project's worktree setup script, or "" when the
// repository ships none. Always read from the main checkout, never from the
// worktree being created: a worktree opened on somebody else's branch (the PR
// flow) must not execute that branch's script.
//
// Every read failure answers "": a script lich cannot read is a script it will
// not run either.
func SetupScript(projectPath string) string {
	data, err := os.ReadFile(filepath.Join(projectPath, filepath.FromSlash(SetupScriptPath)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
