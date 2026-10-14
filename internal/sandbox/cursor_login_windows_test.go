package sandbox

import "path/filepath"

// cursorDefaultDirs is what Cursor's state is with no variable set. On Windows
// the login is in the roaming AppData.
func cursorDefaultDirs(home string) []string {
	return []string{filepath.Join(home, ".cursor"), filepath.Join(home, "AppData", "Roaming", "Cursor")}
}

// cursorXDGDirs is its state under XDG_CONFIG_HOME, which moves only the config
// dir.
func cursorXDGDirs(home, config string) []string {
	return []string{
		filepath.Join(config, "cursor"),
		filepath.Join(home, ".cursor"),
		filepath.Join(home, "AppData", "Roaming", "Cursor"),
	}
}
