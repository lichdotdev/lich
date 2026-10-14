package sandbox

import "path/filepath"

// cursorDefaultDirs is what a confined Cursor session binds with no variable
// set. On macOS the login file is in ~/.cursor, the config dir itself.
func cursorDefaultDirs(home string) []string {
	return []string{filepath.Join(home, ".cursor")}
}

// cursorXDGDirs is what it binds under XDG_CONFIG_HOME, which moves the config
// dir and leaves the login in ~/.cursor.
func cursorXDGDirs(home, config string) []string {
	return []string{filepath.Join(config, "cursor"), filepath.Join(home, ".cursor")}
}
