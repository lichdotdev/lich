//go:build !windows

package claudeprofile

import "os"

// linkDir links path to the directory target.
func linkDir(target, path string) error { return os.Symlink(target, path) }

// linkFile links path to the file target. Off Windows a link never falls back
// to a copy, so copied is always false.
func linkFile(target, path string) (copied bool, err error) {
	return false, os.Symlink(target, path)
}

// isLink reports whether path is a link this package made.
func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
