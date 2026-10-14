package providers

import "path/filepath"

// CursorAuthDirIn is CursorAuthDir answered against another process's
// environment. On Windows the login is in the roaming AppData, under a
// title-cased "Cursor".
func CursorAuthDirIn(getenv func(string) string, home string) string {
	base := filepath.Join(home, "AppData", "Roaming")
	if dir := getenv("APPDATA"); filepath.IsAbs(dir) {
		base = dir
	}
	return filepath.Join(base, "Cursor")
}
