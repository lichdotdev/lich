package shellintegration

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func envOf(env []string, key string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

func TestApplyPointsEachShellAtItsScripts(t *testing.T) {
	dir := filepath.Join("cfg", "shell-integration")
	t.Run("zsh", func(t *testing.T) {
		args, env := Apply("/usr/bin/zsh", nil, []string{"HOME=/h"}, dir)
		if len(args) != 0 {
			t.Errorf("args = %q, want none", args)
		}
		if got, _ := envOf(env, "ZDOTDIR"); got != filepath.Join(dir, "zsh") {
			t.Errorf("ZDOTDIR = %q", got)
		}
		if _, ok := envOf(env, "LICH_USER_ZDOTDIR"); ok {
			t.Error("LICH_USER_ZDOTDIR set for a user with no ZDOTDIR")
		}
	})
	t.Run("zsh keeps the user's ZDOTDIR", func(t *testing.T) {
		_, env := Apply("zsh", nil, []string{"ZDOTDIR=/h/.config/zsh"}, dir)
		if got, _ := envOf(env, "LICH_USER_ZDOTDIR"); got != "/h/.config/zsh" {
			t.Errorf("LICH_USER_ZDOTDIR = %q", got)
		}
		if got, _ := envOf(env, "ZDOTDIR"); got != filepath.Join(dir, "zsh") {
			t.Errorf("ZDOTDIR = %q", got)
		}
	})
	t.Run("bash", func(t *testing.T) {
		args, _ := Apply("/bin/bash", []string{"-l"}, nil, dir)
		want := []string{"--rcfile", filepath.Join(dir, "bash", "lich.bash"), "-l"}
		if !slices.Equal(args, want) {
			t.Errorf("args = %q, want %q", args, want)
		}
	})
	t.Run("fish", func(t *testing.T) {
		_, env := Apply("fish", nil, []string{"XDG_DATA_DIRS=/usr/share"}, dir)
		if got, _ := envOf(env, "XDG_DATA_DIRS"); got != dir+string(os.PathListSeparator)+"/usr/share" {
			t.Errorf("XDG_DATA_DIRS = %q", got)
		}
		if got, _ := envOf(env, "LICH_SHELL_INTEGRATION_XDG_DIR"); got != dir {
			t.Errorf("LICH_SHELL_INTEGRATION_XDG_DIR = %q", got)
		}
	})
	t.Run("fish without XDG_DATA_DIRS keeps the spec's default", func(t *testing.T) {
		_, env := Apply("fish", nil, nil, dir)
		if got, _ := envOf(env, "XDG_DATA_DIRS"); got != dir+string(os.PathListSeparator)+"/usr/local/share:/usr/share" {
			t.Errorf("XDG_DATA_DIRS = %q", got)
		}
	})
	for _, shell := range []string{"/bin/sh", "pwsh.exe", `C:\Windows\powershell.exe`} {
		t.Run(shell+" is left alone", func(t *testing.T) {
			args, env := Apply(shell, []string{"-x"}, []string{"A=1"}, dir)
			if !slices.Equal(args, []string{"-x"}) || !slices.Equal(env, []string{"A=1"}) {
				t.Errorf("Apply changed %s: %q %q", shell, args, env)
			}
		})
	}
}

// runInteractive runs shell interactive with the integration, in a home of its
// own holding rcName, and returns what it printed for two prompts and a command.
func runInteractive(t *testing.T, shell, rcName string) string {
	t.Helper()
	bin, err := exec.LookPath(shell)
	if err != nil {
		t.Skipf("%s not installed", shell)
	}
	dir := t.TempDir()
	if err := Install(dir); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, rcName), []byte("echo user-rc-ran\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args, env := Apply(bin, []string{"-i"}, []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "TERM=dumb"}, dir)
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader("true\nexit\n")
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestShellsMarkTheirPromptAndStillReadTheUsersRC(t *testing.T) {
	cases := []struct{ shell, rc, mark string }{
		{"zsh", ".zshrc", "\x1b]133;A;redraw=1\a"},
		{"bash", ".bashrc", "\x1b]133;A;redraw=last\a"},
	}
	for _, c := range cases {
		t.Run(c.shell, func(t *testing.T) {
			out := runInteractive(t, c.shell, c.rc)
			for _, want := range []string{"user-rc-ran", c.mark, "\x1b]133;B\a", "\x1b]133;C\a", "\x1b]133;D;0\a"} {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%q", want, out)
				}
			}
		})
	}
}
