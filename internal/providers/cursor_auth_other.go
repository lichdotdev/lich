//go:build !darwin && !windows

package providers

import "path/filepath"

// CursorAuthDirIn is CursorAuthDir answered against another process's
// environment. Here the login is xdg-basedir: $XDG_CONFIG_HOME/cursor, else
// ~/.config/cursor.
func CursorAuthDirIn(getenv func(string) string, home string) string {
	base := filepath.Join(home, ".config")
	if dir := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		base = dir
	}
	return filepath.Join(base, "cursor")
}
