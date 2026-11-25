//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddToShellRCWritesTheShellsOwnFile(t *testing.T) {
	t.Setenv("ZDOTDIR", "")
	cases := map[string]struct{ file, line string }{
		"/usr/bin/zsh":  {".zshrc", `export PATH="$HOME/.local/bin:$PATH"`},
		"/bin/sh":       {".profile", `export PATH="$HOME/.local/bin:$PATH"`},
		"/usr/bin/fish": {".config/fish/conf.d/lich-path.fish", "contains $HOME/.local/bin $PATH; or set -gx PATH $HOME/.local/bin $PATH"},
	}
	for shell, want := range cases {
		home := t.TempDir()
		dir := filepath.Join(home, ".local", "bin")
		file, err := AddToShellRC(shell, home, dir)
		if err != nil || file != filepath.Join(home, want.file) {
			t.Fatalf("%s: wrote %q, %v", shell, file, err)
		}
		data, _ := os.ReadFile(file)
		if !strings.Contains(string(data), rcMarker+"\n"+want.line+"\n") {
			t.Errorf("%s: %q", shell, data)
		}
		if again, err := AddToShellRC(shell, home, dir); again != "" || err != nil {
			t.Errorf("%s: a second add wrote %q, %v", shell, again, err)
		}
	}
}

func TestAddToShellRCLeavesAFileThatHasTheFolder(t *testing.T) {
	home := t.TempDir()
	rc, _ := shellRC("bash", home, "")
	before := "export PATH=\"$HOME/.local/bin:$PATH\"\n"
	if err := os.WriteFile(rc, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	if file, err := AddToShellRC("/bin/bash", home, filepath.Join(home, ".local", "bin")); file != "" || err != nil {
		t.Fatalf("wrote %q, %v", file, err)
	}
	if data, _ := os.ReadFile(rc); string(data) != before {
		t.Errorf("rc changed: %q", data)
	}
}

func TestAddToShellRCFollowsZDOTDIR(t *testing.T) {
	home, zdot := t.TempDir(), t.TempDir()
	t.Setenv("ZDOTDIR", zdot)
	if file, err := AddToShellRC("zsh", home, "/opt/bin"); file != filepath.Join(zdot, ".zshrc") || err != nil {
		t.Fatalf("wrote %q, %v", file, err)
	}
}
