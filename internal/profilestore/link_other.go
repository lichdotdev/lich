//go:build !windows

package profilestore

import (
	"io/fs"
	"os"
)

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

// linksDir reports whether the link path stands for a directory. Off Windows
// either answer makes the same symlink, so a dangling one reads as a file.
func linksDir(path string, _ fs.FileInfo) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// linkMissingFile makes path a symlink to target, a file that does not exist.
func linkMissingFile(target, path string) error { return os.Symlink(target, path) }

// linkHomeFile links a link home's path to Personal's file target.
func linkHomeFile(target, path string) error { return os.Symlink(target, path) }
