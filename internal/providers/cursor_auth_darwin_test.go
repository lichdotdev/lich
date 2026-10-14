package providers

import (
	"path/filepath"
	"testing"
)

// On macOS the login file is ~/.cursor/auth.json whatever is set: neither
// variable that moves the config dir moves it.
func TestCursorAuthDirIgnoresTheVariablesOnMacOS(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CURSOR_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, want := CursorAuthDir(home), filepath.Join(home, ".cursor"); got != want {
		t.Errorf("CursorAuthDir = %q, want %q", got, want)
	}
}
