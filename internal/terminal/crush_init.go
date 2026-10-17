package terminal

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/providers"
)

// markCrushInitialized writes the marker Crush reads as "this project was
// already offered its init", so a session does not open on the "initialize
// this project?" question, whose Enter runs an init turn. Crush keeps the
// marker in `.crush/` at the checkout's top, or the folder itself outside a
// repository, so every new worktree asked again (measured on v0.98.1). The
// offer to write AGENTS.md goes with it, in every session. A failure costs the
// session the question, never its spawn.
func markCrushInitialized(kind, cwd string) {
	if kind != providers.Crush || cwd == "" {
		return
	}
	root, _ := project.CheckoutRoots(cwd)
	if root == "" {
		root = cwd
	}
	if err := writeCrushInit(filepath.Join(root, ".crush")); err != nil {
		slog.Warn("mark the checkout initialized for Crush", "cwd", cwd, "err", err)
	}
}

// writeCrushInit creates the empty init marker and the `*` .gitignore Crush
// puts beside it on its first start, so the folder never shows in git status
// even when Crush does not get that far. A file already there is left alone.
func writeCrushInit(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for name, content := range map[string]string{".gitignore": "*\n", "init": ""} {
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("create %s: %w", path, err)
		}
		_, err = f.WriteString(content)
		if err := errors.Join(err, f.Close()); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
