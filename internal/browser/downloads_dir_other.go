//go:build !linux

package browser

import (
	"os"
	"path/filepath"
)

// downloadDir is ~/Downloads, where macOS keeps it and Windows puts it by
// default.
// shortcut: a Windows user who moved the Downloads known folder still gets
// %USERPROFILE%\Downloads; read SHGetKnownFolderPath if someone asks.
func downloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}
