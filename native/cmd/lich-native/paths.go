package main

import (
	"regexp"
	"strings"

	"github.com/omartelo/lich/native/i18n"
)

// homeRoot is frontend/src/lib/paths.ts's HOME_ROOTS.
var homeRoot = regexp.MustCompile(`^(?:/home/[^/]+|/Users/[^/]+|[A-Za-z]:\\Users\\[^\\]+)`)

// displayPath collapses a home directory prefix to "~", as shells render it.
func displayPath(path string) string {
	return homeRoot.ReplaceAllLiteralString(path, "~")
}

// shortPathParts is how many of a long path's last parts shortPath keeps:
// enough to tell one worktree from another by its hash and name.
const shortPathParts = 3

// shortPath is displayPath cut to its last shortPathParts parts behind "…"
// when it has more, so a worktree reads as …worktrees/<hash>/<name> rather
// than the whole of lich's data directory.
func shortPath(path string) string {
	shown := displayPath(path)
	sep := "/"
	if strings.Contains(shown, `\`) && !strings.Contains(shown, "/") {
		sep = `\`
	}
	parts := strings.Split(strings.TrimPrefix(shown, sep), sep)
	if len(parts) <= shortPathParts {
		return shown
	}
	return "…" + strings.Join(parts[len(parts)-shortPathParts:], sep)
}

// unknownCwd is the path readout when the shell went somewhere the backend
// cannot see into (tmux, ssh, a container): the last local path is a
// directory the user has left. Mirrors env.paths.cwdUnknown.
func unknownCwd(host string) string {
	return i18n.T("env.paths.cwdUnknown", i18n.Args{"host": host})
}
