package main

import (
	"context"
	"regexp"
	"strings"

	"github.com/lichdotdev/lich/native/backend"
)

// attachFile is the footer's attach button. The
// picker runs on the backend: a confined session cannot open a file outside
// its checkout, so the call that chooses the file also copies it where the
// session can read it. checkout, not the shell's cwd, because the sandbox is
// built around the spawn directory. A cancelled dialog is no error; a failure
// carries the backend's sentence, which names why a file was refused. write
// puts the paste at the prompt.
func attachFile(ctx context.Context, c *backend.Client, write func(ctx context.Context, id, block string) error, sessionID, checkout string, sandboxed, windows bool) error {
	a, err := c.Attach(ctx, sessionID, checkout, sandboxed)
	if err != nil || a.Path == "" {
		return err
	}
	return write(ctx, sessionID, composeDroppedPaths([]string{a.Path}, windows, []string{a.Notice}))
}

// composeDroppedPaths returns the paths quoted and space-separated as a
// terminal pastes a drop, with a trailing space so what is typed next
// does not run into the last path. Notices, the backend's lines about paths
// that are copies', go under the paths inside the same paste. "" for no paths.
func composeDroppedPaths(paths []string, windows bool, notices []string) string {
	if len(paths) == 0 {
		return ""
	}
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = quotePath(p, windows)
	}
	line := strings.Join(quoted, " ")
	var lines []string
	for _, n := range notices {
		if n != "" {
			lines = append(lines, n)
		}
	}
	if len(lines) == 0 {
		return bracketedPaste(line + " ")
	}
	return bracketedPaste(line + "\n" + strings.Join(lines, "\n") + "\n")
}

var shellSafePath = regexp.MustCompile(`^[\w@%+=:,./\\-]+$`)

// quotePath keeps a path one argument in whatever shell reads the prompt.
// POSIX closes the quoting to escape a quote; PowerShell, what a Windows
// session runs, doubles it. Mirrors internal/shquote.
func quotePath(path string, windows bool) string {
	if shellSafePath.MatchString(path) {
		return path
	}
	if windows {
		return "'" + strings.ReplaceAll(path, "'", "''") + "'"
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// bracketedPaste wraps text as one paste, which leaves it unsent at the
// prompt. It is a trust boundary: a file name is written by strangers, and an
// ESC[201~ inside it would close the block early and run what follows as
// keys. So, as internal/relay/compose.go sanitize does, \n and \t stay and the
// rest of C0 and DEL go; \r goes too, being a submit in some readers.
func bracketedPaste(text string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || (r >= 0x20 && r != 0x7f) {
			return r
		}
		return -1
	}, text)
	return "\x1b[200~" + clean + "\x1b[201~"
}
