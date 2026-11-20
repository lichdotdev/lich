//go:build !windows

package terminal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// rcMarker opens the lines lich adds to a shell's startup file, so a reader
// of the file knows where they came from.
const rcMarker = "# Added by lich: where a provider's installer put its binary"

// AddToShellRC puts dir on the PATH of the user's shell, in the startup file
// that shell reads, so the binary an installer left there answers in any
// terminal and in the login shell lich resolves its PATH from at the next
// launch. A file that already names dir is left alone. It returns the file it
// wrote, "" when it had nothing to do.
func AddToShellRC(shell, home, dir string) (string, error) {
	file, line := shellRC(filepath.Base(shell), home, homeRelative(home, dir))
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if strings.Contains(string(data), dir) || strings.Contains(string(data), homeRelative(home, dir)) {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	_, err = fmt.Fprintf(f, "\n%s\n%s\n", rcMarker, line)
	return file, errors.Join(err, f.Close())
}

// shellRC is the file shell reads at startup and the line that puts dir on
// its PATH there: fish's own syntax in a conf.d file of lich's, zsh's file
// under ZDOTDIR, bash's .bash_profile on macOS (Terminal opens login shells)
// and .bashrc elsewhere, and .profile for any other shell.
func shellRC(shell, home, dir string) (file, line string) {
	switch shell {
	case "fish":
		return filepath.Join(home, ".config", "fish", "conf.d", "lich-path.fish"),
			fmt.Sprintf("contains %s $PATH; or set -gx PATH %s $PATH", dir, dir)
	case "zsh":
		root := os.Getenv("ZDOTDIR")
		if root == "" {
			root = home
		}
		return filepath.Join(root, ".zshrc"), exportLine(dir)
	case "bash":
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile"), exportLine(dir)
		}
		return filepath.Join(home, ".bashrc"), exportLine(dir)
	}
	return filepath.Join(home, ".profile"), exportLine(dir)
}

func exportLine(dir string) string { return fmt.Sprintf(`export PATH="%s:$PATH"`, dir) }

// homeRelative spells a folder under home from $HOME, as the vendors' own
// instructions do.
func homeRelative(home, dir string) string {
	if rest, ok := strings.CutPrefix(dir, home+string(filepath.Separator)); ok {
		return "$HOME/" + filepath.ToSlash(rest)
	}
	return dir
}
