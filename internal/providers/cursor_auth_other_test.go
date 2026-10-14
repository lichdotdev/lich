//go:build !darwin && !windows

package providers

import (
	"path/filepath"
	"testing"
)

// The login is xdg-basedir even though the config dir is not: with no
// XDG_CONFIG_HOME it is ~/.config/cursor, never the ~/.cursor the config dir
// falls back to, and CURSOR_CONFIG_DIR does not move it (issue #765).
func TestCursorAuthDirIsXDGBasedir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CURSOR_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	if got, want := CursorAuthDir(home), filepath.Join(home, ".config", "cursor"); got != want {
		t.Errorf("with no XDG_CONFIG_HOME = %q, want %q", got, want)
	}

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got, want := CursorAuthDir(home), filepath.Join(xdg, "cursor"); got != want {
		t.Errorf("with XDG_CONFIG_HOME = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join("relative", "config"))
	if got, want := CursorAuthDir(home), filepath.Join(home, ".config", "cursor"); got != want {
		t.Errorf("a relative XDG_CONFIG_HOME = %q, want %q", got, want)
	}
}
