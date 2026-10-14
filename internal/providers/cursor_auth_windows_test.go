package providers

import (
	"path/filepath"
	"testing"
)

// On Windows the login is under the roaming AppData, in a "Cursor" folder, and
// the variables that move the config dir leave it alone.
func TestCursorAuthDirIsUnderRoamingAppData(t *testing.T) {
	home := t.TempDir()
	appData := t.TempDir()
	t.Setenv("CURSOR_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", appData)
	if got, want := CursorAuthDir(home), filepath.Join(appData, "Cursor"); got != want {
		t.Errorf("with APPDATA = %q, want %q", got, want)
	}

	t.Setenv("APPDATA", "")
	if got, want := CursorAuthDir(home), filepath.Join(home, "AppData", "Roaming", "Cursor"); got != want {
		t.Errorf("with no APPDATA = %q, want %q", got, want)
	}
}
