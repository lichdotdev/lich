package terminal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/profilestore"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
)

// cursorTrustMarker is the file whose presence under a folder's project dir
// means Cursor CLI trusts that folder (read in the 2026.10.16 bundle,
// src/workspace/approval.tsx). Its content is never read.
const cursorTrustMarker = ".workspace-trusted"

// cursorMinTrustDepth is the fewest path components a parent folder needs for
// its marker to count for the folders under it, the bundle's own floor.
const cursorMinTrustDepth = 3

var cursorSlugRun = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// inheritCursorTrust marks a linked worktree trusted for Cursor CLI when the
// person trusted its main checkout. Cursor finds a trusted folder in the folder
// itself or a parent, and a worktree sits beside its repository rather than
// under it, so every worktree lich makes asked again (measured on 2026.10.01
// and 2026.10.16). It never trusts a folder nobody trusted, and a failure
// costs the session the question, never its spawn.
func inheritCursorTrust(kind, cwd string) {
	if kind != providers.Cursor || cwd == "" {
		return
	}
	top, origin := project.CheckoutRoots(cwd)
	if top == "" || origin == "" || filepath.Clean(top) == filepath.Clean(origin) {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("carry folder trust into Cursor CLI", "cwd", cwd, "err", err)
		return
	}
	projects := cursorProjectsDir(home)
	if cursorTrusts(projects, home, top) || !cursorTrusts(projects, home, origin) {
		return
	}
	if err := writeCursorTrust(projects, top); err != nil {
		slog.Warn("carry folder trust into Cursor CLI", "cwd", cwd, "err", err)
	}
}

// cursorProjectsDir is where Cursor CLI keeps a folder of its own per project:
// under CURSOR_DATA_DIR, else ~/.cursor. Neither CURSOR_CONFIG_DIR nor a lich
// profile moves it.
func cursorProjectsDir(home string) string {
	if dir := os.Getenv("CURSOR_DATA_DIR"); strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "projects")
	}
	return filepath.Join(home, ".cursor", "projects")
}

// cursorSlug is the name of a folder's project dir: every run of characters
// outside [A-Za-z0-9] becomes one dash, trimmed at both ends.
func cursorSlug(dir string) string {
	return strings.Trim(cursorSlugRun.ReplaceAllString(dir, "-"), "-")
}

// cursorTrusts reports whether Cursor CLI would open in dir without asking: a
// marker for dir, or for a parent that is below home and at least
// cursorMinTrustDepth components deep.
func cursorTrusts(projects, home, dir string) bool {
	if hasCursorMarker(projects, dir) {
		return true
	}
	for parent := filepath.Dir(dir); parent != dir; dir, parent = parent, filepath.Dir(parent) {
		if cursorParentCounts(parent, home) && hasCursorMarker(projects, parent) {
			return true
		}
	}
	return false
}

func cursorParentCounts(dir, home string) bool {
	atOrAboveHome := dir == home || strings.HasPrefix(home, dir+string(filepath.Separator))
	depth := len(strings.FieldsFunc(dir, func(r rune) bool { return r == filepath.Separator }))
	return !atOrAboveHome && depth >= cursorMinTrustDepth
}

func hasCursorMarker(projects, dir string) bool {
	_, err := os.Stat(filepath.Join(projects, cursorSlug(dir), cursorTrustMarker))
	return err == nil
}

// writeCursorTrust writes the marker the way Cursor CLI writes its own, so the
// file reads the same to anyone who opens it.
func writeCursorTrust(projects, dir string) error {
	folder := filepath.Join(projects, cursorSlug(dir))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", folder, err)
	}
	data, err := json.MarshalIndent(map[string]string{
		"trustedAt":     time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"workspacePath": dir,
	}, "", "  ")
	if err != nil {
		return err
	}
	return profilestore.WriteFileAtomic(filepath.Join(folder, cursorTrustMarker), data, 0o644)
}
