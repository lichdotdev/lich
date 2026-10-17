//go:build windows

package profilestore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/lichdotdev/lich/internal/winexec"
	"golang.org/x/sys/windows"
)

// symlink is os.Symlink, swapped in tests for a Windows without Developer Mode.
var symlink = os.Symlink

// LinkDir makes path a junction to target. A junction needs no privilege, where
// a directory symlink needs Developer Mode or an elevated process.
func LinkDir(target, path string) error {
	cmd := exec.Command("cmd", "/c", "mklink", "/J", path, target)
	winexec.Hide(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mklink /J: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// linkFile makes path a symlink to the file target, which Windows allows only
// with Developer Mode on, and copies the file when it refuses. A hard link is
// not the fallback: a CLI that replaces the file on write leaves the two
// diverged anyway (measured on Claude Code). A relative target is read from
// beside the link, as the symlink would have resolved it.
func linkFile(target, path string) (copied bool, err error) {
	if symlink(target, path) == nil {
		return false, nil
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return true, copyFile(target, path)
}

// IsLink reports whether path is a link this package made: a symlink, or a
// junction, which Go reports as an irregular file rather than a symlink.
func IsLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// linksDir reports whether the link path, which info is the Lstat of, stands
// for a directory. Windows records that on the link itself, so it holds when
// the target is gone: a junction, or a symlink made with the directory flag.
func linksDir(_ string, info fs.FileInfo) bool {
	if info.Mode()&os.ModeIrregular != 0 {
		return true
	}
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attrs.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0
}

// linkMissingFile makes path a symlink to target, a file that does not exist.
// Without Developer Mode Windows refuses it and there is no file to copy
// instead, so the link is left out: its target was all it held.
func linkMissingFile(target, path string) error {
	err := symlink(target, path)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		return nil
	}
	return err
}

// linkHomeFile links a link home's path to Personal's file target. Without
// Developer Mode Windows refuses a file symlink, and the file is left out
// rather than copied: a copy would be the profile's own from then on and miss
// every later change to Personal's, where a missing entry is linked by the
// first spawn after Developer Mode is on.
func linkHomeFile(target, path string) error {
	err := symlink(target, path)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		return nil
	}
	return err
}
