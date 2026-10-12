//go:build windows

package claudeprofile

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/omartelo/lich/internal/winexec"
)

// linkDir makes path a junction to target. A junction needs no privilege, where
// a directory symlink needs Developer Mode or an elevated process.
func linkDir(target, path string) error {
	cmd := exec.Command("cmd", "/c", "mklink", "/J", path, target)
	winexec.Hide(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mklink /J: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// linkFile makes path a symlink to the file target, which Windows allows only
// with Developer Mode on, and copies the file when it refuses. A hard link is
// not the fallback: Claude Code replaces the file on its first write and the two
// diverge anyway (measured).
func linkFile(target, path string) (copied bool, err error) {
	if os.Symlink(target, path) == nil {
		return false, nil
	}
	return true, copyFile(target, path)
}

// isLink reports whether path is a link this package made: a symlink, or a
// junction, which Go reports as an irregular file rather than a symlink.
func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}
