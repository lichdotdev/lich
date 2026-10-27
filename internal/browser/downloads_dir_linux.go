//go:build linux

package browser

import (
	"os"
	"path/filepath"
	"strings"
)

// downloadDir is the folder a desktop names Downloads: XDG_DOWNLOAD_DIR, set
// in the environment or in user-dirs.dirs (where xdg-user-dirs writes it,
// translated on a desktop in another language), else ~/Downloads.
func downloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if dir := os.Getenv("XDG_DOWNLOAD_DIR"); filepath.IsAbs(dir) {
		return dir, nil
	}
	conf := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(conf) {
		conf = filepath.Join(home, ".config")
	}
	if b, err := os.ReadFile(filepath.Join(conf, "user-dirs.dirs")); err == nil {
		if dir := userDirsDownload(string(b), home); dir != "" {
			return dir, nil
		}
	}
	return filepath.Join(home, "Downloads"), nil
}

// userDirsDownload reads XDG_DOWNLOAD_DIR out of a user-dirs.dirs, whose
// values are "$HOME/…" or an absolute path; "" when it names neither.
func userDirsDownload(text, home string) string {
	for _, line := range strings.Split(text, "\n") {
		v, ok := strings.CutPrefix(strings.TrimSpace(line), "XDG_DOWNLOAD_DIR=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		if rest, ok := strings.CutPrefix(v, "$HOME"); ok {
			v = home + rest
		}
		if filepath.IsAbs(v) {
			return v
		}
		return ""
	}
	return ""
}
