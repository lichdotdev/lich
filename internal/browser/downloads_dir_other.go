//go:build !linux && !windows

package browser

import (
	"os"
	"path/filepath"
)

// downloadDir is ~/Downloads, where macOS keeps it.
func downloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}
