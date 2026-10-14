// Package shellintegration marks a shell session's prompt for the terminal
// with OSC 133, the way Ghostty's shell integration does, so the native
// window's terminal (libghostty-vt) clears the prompt on a resize instead of
// leaving the lines a reflow wrapped above the one the shell draws again.
//
// The scripts are written to disk once at startup (Install) and the shell is
// pointed at them when its session spawns (Apply): zsh through ZDOTDIR, bash
// through --rcfile, fish through XDG_DATA_DIRS. Each one gives back what it
// borrowed and reads the user's own rc files as the shell would have.
package shellintegration

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:scripts
var scripts embed.FS

// fishDefaultDataDirs is XDG_DATA_DIRS when unset, per the XDG Base Directory
// spec: prepending to nothing would hide fish's own vendor directories.
const fishDefaultDataDirs = "/usr/local/share:/usr/share"

// Dir is where Install writes the scripts under lich's config directory, which
// a confined session's sandbox mounts read-only.
func Dir(configDir string) string {
	return filepath.Join(configDir, "lich", "shell-integration")
}

// Install writes the scripts into dir, replacing what an older lich wrote.
func Install(dir string) error {
	return fs.WalkDir(scripts, "scripts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(path, "scripts")))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := scripts.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("install shell integration: %w", err)
		}
		return nil
	})
}

// Apply returns the arguments and environment that start shell with the
// integration in dir. A shell it has no script for (sh, PowerShell) is
// returned as it came.
func Apply(shell string, args, env []string, dir string) ([]string, []string) {
	switch shellName(shell) {
	case "zsh":
		if user, ok := lookup(env, "ZDOTDIR"); ok {
			env = set(env, "LICH_USER_ZDOTDIR", user)
		}
		return args, set(env, "ZDOTDIR", filepath.Join(dir, "zsh"))
	case "bash":
		return append([]string{"--rcfile", filepath.Join(dir, "bash", "lich.bash")}, args...), env
	case "fish":
		dataDirs, ok := lookup(env, "XDG_DATA_DIRS")
		if !ok || dataDirs == "" {
			dataDirs = fishDefaultDataDirs
		}
		env = set(env, "LICH_SHELL_INTEGRATION_XDG_DIR", dir)
		return args, set(env, "XDG_DATA_DIRS", dir+string(os.PathListSeparator)+dataDirs)
	}
	return args, env
}

// shellName is the shell's executable name, whichever separator its path
// uses.
func shellName(shell string) string {
	base := shell[strings.LastIndexAny(shell, `/\`)+1:]
	return strings.TrimSuffix(strings.ToLower(base), ".exe")
}

func lookup(env []string, key string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

// set returns env with key set to value, replacing an earlier entry.
func set(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != key {
			out = append(out, kv)
		}
	}
	return append(out, key+"="+value)
}
