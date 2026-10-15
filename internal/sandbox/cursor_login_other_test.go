//go:build !darwin && !windows

package sandbox

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// cursorDefaultDirs is what a confined Cursor session binds with no variable
// set: its config dir, which is ~/.cursor, and its login in ~/.config/cursor.
func cursorDefaultDirs(home string) []string {
	return []string{filepath.Join(home, ".cursor"), filepath.Join(home, ".config", "cursor")}
}

// cursorXDGDirs is what it binds under XDG_CONFIG_HOME, where the login and the
// config dir are the same directory.
func cursorXDGDirs(home, config string) []string {
	return []string{filepath.Join(config, "cursor"), filepath.Join(home, ".cursor")}
}

// With no XDG_CONFIG_HOME Cursor's login is in ~/.config/cursor, beside neither
// of the two directories the rest of its state is in, so a confined session
// bound without it opens at the login prompt (issue #765).
func TestStateDirsBindsTheCursorLogin(t *testing.T) {
	clearHarnessEnv(t)
	home := testHome()
	if got := stateDirs(providers.Cursor, home); !slices.Contains(got, filepath.Join(home, ".config", "cursor")) {
		t.Errorf("stateDirs(cursor) = %v, want ~/.config/cursor among them", got)
	}
}
