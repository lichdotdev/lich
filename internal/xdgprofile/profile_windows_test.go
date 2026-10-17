//go:build windows

package xdgprofile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// windowsMachine points the home, %APPDATA% and lich's config dir at one
// scratch root holding some other tool's state and Personal's own for opencode
// and Cursor CLI, and returns the home and %APPDATA%.
func windowsMachine(t *testing.T) (home, appData string) {
	t.Helper()
	home = t.TempDir()
	appData = filepath.Join(home, "AppData", "Roaming")
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", appData)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("CURSOR_CONFIG_DIR", "")
	t.Setenv("LICH_DEV", "")
	if dir, err := os.UserConfigDir(); err != nil || !strings.HasPrefix(dir, home) {
		t.Fatalf("lich's config dir is %s (%v), outside the scratch home", dir, err)
	}
	for _, f := range []string{
		filepath.Join(appData, "GitHub CLI", "hosts.yml"),
		filepath.Join(appData, "Cursor", "auth.json"),
		filepath.Join(appData, "toplevel.ini"),
		filepath.Join(home, ".cursor", "cli-config.json"),
		filepath.Join(home, ".local", "share", "mise", "installs"),
		filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
	} {
		mustWrite(t, f, "personal")
	}
	return home, appData
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProvidersAreOpenCodeAndCursorOnWindows(t *testing.T) {
	var ids []string
	for _, p := range Providers() {
		ids = append(ids, p.ID)
	}
	if !slices.Equal(ids, []string{"opencode", "cursor"}) || Of("kiro") != nil {
		t.Errorf("Providers = %v", ids)
	}
}

func TestAWindowsCursorProfileMovesAppDataAndItsConfigDir(t *testing.T) {
	home, appData := windowsMachine(t)
	p, err := Cursor.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p.Dir, filepath.Join(appData, "lich", "profiles", "cursor")) {
		t.Errorf("the profile lives at %s", p.Dir)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "GitHub CLI")) {
		t.Error("GitHub CLI's folder is not linked")
	}
	for _, name := range []string{"Cursor", "lich"} {
		if _, err := os.Lstat(filepath.Join(p.Dir, name)); err == nil {
			t.Errorf("a new profile holds %s", name)
		}
	}
	if info, err := os.Lstat(filepath.Join(p.Dir, "toplevel.ini")); err == nil && info.Mode().IsRegular() {
		t.Error("a file of APPDATA was copied into the profile instead of linked")
	}
	env := Cursor.Env([]string{"APPDATA=" + appData, "CURSOR_CONFIG_DIR=elsewhere", "PATH=x"}, p.Dir)
	want := []string{"PATH=x", "APPDATA=" + p.Dir, "CURSOR_CONFIG_DIR=" + filepath.Join(p.Dir, "Cursor")}
	if !slices.Equal(env, want) {
		t.Errorf("Env = %v, want %v", env, want)
	}
	states := Cursor.States()
	if len(states) != 2 || states[0].Dir != filepath.Join(home, ".cursor") || states[1].Dir != filepath.Join(p.Dir, "Cursor") {
		t.Errorf("states = %+v", states)
	}
	if err := Cursor.Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join("GitHub CLI", "hosts.yml"), filepath.Join("Cursor", "auth.json"), "toplevel.ini"} {
		if _, err := os.Stat(filepath.Join(appData, f)); err != nil {
			t.Errorf("deleting the profile took Personal's %s: %v", f, err)
		}
	}
}

func TestAWindowsCursorLoginRunsInTheProfile(t *testing.T) {
	windowsMachine(t)
	s := New(Cursor, fakeBins{})
	p, err := s.CreateProfile("Work")
	if err != nil {
		t.Fatal(err)
	}
	line, err := s.LoginCommand(p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{
		"Env:CURSOR_CONFIG_DIR", "$env:APPDATA = '" + p.Dir + "'",
		"$env:CURSOR_CONFIG_DIR = '" + filepath.Join(p.Dir, "Cursor") + "'", " login",
	} {
		if !strings.Contains(line, part) {
			t.Errorf("login line %q lacks %q", line, part)
		}
	}
}

func TestAWindowsOpenCodeProfileLinksTheDataHome(t *testing.T) {
	home, _ := windowsMachine(t)
	p, err := OpenCode.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "mise")) {
		t.Error("mise is not linked")
	}
	if _, err := os.Lstat(filepath.Join(p.Dir, "opencode")); err == nil {
		t.Error("a new profile holds opencode's folder")
	}
	if env := OpenCode.Env(nil, p.Dir); !slices.Equal(env, []string{"XDG_DATA_HOME=" + p.Dir}) {
		t.Errorf("Env = %v", env)
	}
	if states := OpenCode.States(); states[0].Dir != filepath.Join(home, ".local", "share", "opencode") {
		t.Errorf("Personal's state = %s", states[0].Dir)
	}
}

type fakeBins struct{}

func (fakeBins) ProviderBin(_, _ string) string { return `C:\bin\cursor-agent.exe` }
