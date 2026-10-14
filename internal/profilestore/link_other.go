//go:build !windows

package profilestore

import "os"

// LinkDir links path to the directory target.
func LinkDir(target, path string) error { return os.Symlink(target, path) }

// linkFile links path to the file target. Off Windows a link never falls back
// to a copy, so copied is always false.
func linkFile(target, path string) (copied bool, err error) {
	return false, os.Symlink(target, path)
}

// IsLink reports whether path is a link this package made.
func IsLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}
