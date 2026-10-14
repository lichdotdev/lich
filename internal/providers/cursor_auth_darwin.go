package providers

import "path/filepath"

// CursorAuthDirIn is CursorAuthDir answered against another process's
// environment. On macOS no variable moves the login: it is ~/.cursor/auth.json,
// when it is not in the Keychain.
func CursorAuthDirIn(_ func(string) string, home string) string {
	return filepath.Join(home, ".cursor")
}
