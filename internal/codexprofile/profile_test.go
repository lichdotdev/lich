package codexprofile

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/profilestore"
)

// machine points every directory this package resolves at a scratch home: the
// user's config dir (XDG on Linux, APPDATA on Windows, the home on macOS) and
// Personal's ~/.codex under it. It returns Personal's directory.
func machine(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv(HomeVar, "")
	t.Setenv("LICH_DEV", "")
	personal := filepath.Join(home, ".codex")
	mustMkdir(t, personal)
	return personal
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCreateLaysTheProfileOutAgainstPersonal(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "config.toml"), "model = \"x\"\n\n[plugins.\"a@m\"]\nenabled = true\n")
	mustWrite(t, filepath.Join(personal, "auth.json"), `{"secret":true}`)
	mustWrite(t, filepath.Join(personal, "plugins", "cache", "m", "a", "file"), "x")
	mustWrite(t, filepath.Join(personal, "history.jsonl"), "{}\n")
	mustMkdir(t, filepath.Join(personal, "skills"))
	mustMkdir(t, filepath.Join(personal, "sessions", "2026"))

	p, err := Create("Work Account")
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "work-account" || p.Name != "Work Account" {
		t.Fatalf("created %+v", p)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "skills")) {
		t.Error("skills is not linked to Personal")
	}
	config := filepath.Join(p.Dir, "config.toml")
	if profilestore.IsLink(config) {
		t.Error("config.toml is linked to Personal, which shares the plugin set")
	}
	if data, _ := os.ReadFile(config); string(data) != "model = \"x\"\n\n" {
		t.Errorf("config.toml = %q, want Personal's settings without its plugins", data)
	}
	for _, own := range []string{"auth.json", "history.jsonl", "sessions", "plugins"} {
		if _, err := os.Lstat(filepath.Join(p.Dir, own)); !os.IsNotExist(err) {
			t.Errorf("%s reached the new profile: %v", own, err)
		}
	}

	profiles, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || !profiles[0].Personal || profiles[0].Dir != personal || profiles[1].Slug != p.Slug {
		t.Errorf("listed %+v", profiles)
	}
}

func TestSlugsAreFreeAndNeverPersonal(t *testing.T) {
	machine(t)
	cases := []struct{ name, want string }{
		{"Work", "work"},
		{"work", "work-2"},
		{"Personal", "personal-2"},
		{"!!!", "profile"},
		{strings.Repeat("a", 50), strings.Repeat("a", 32)},
	}
	for _, c := range cases {
		p, err := Create(c.name)
		if err != nil {
			t.Fatal(err)
		}
		if p.Slug != c.want {
			t.Errorf("Create(%q) slug %q, want %q", c.name, p.Slug, c.want)
		}
	}
	if _, err := Create("   "); err == nil {
		t.Error("a blank name was accepted")
	}
}

func TestRenameKeepsTheDirectory(t *testing.T) {
	machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := Rename(p.Slug, "Client"); err != nil {
		t.Fatal(err)
	}
	profiles, _ := List()
	if profiles[1].Name != "Client" || profiles[1].Dir != p.Dir {
		t.Errorf("after rename %+v, want name Client at %s", profiles[1], p.Dir)
	}
	if err := Rename(p.Slug, " "); err == nil {
		t.Error("a blank name was accepted")
	}
}

func TestRenamingPersonalNeverTouchesItsDirectory(t *testing.T) {
	personal := machine(t)
	if err := Rename(Personal, "Mine"); err != nil {
		t.Fatal(err)
	}
	profiles, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if profiles[0].Name != "Mine" || !profiles[0].Personal || profiles[0].Dir != personal {
		t.Errorf("after rename %+v, want Personal named Mine at %s", profiles[0], personal)
	}
	entries, _ := os.ReadDir(personal)
	if len(entries) != 0 {
		t.Errorf("the rename wrote into Personal's directory: %v", entries)
	}
	if err := Rename(Personal, " "); err == nil {
		t.Error("a blank name was accepted")
	}
}

func TestDeleteNeverReachesPersonal(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "config.toml"), "")
	mustWrite(t, filepath.Join(personal, "plugins", "cache", "m", "p", "file"), "x")
	mustWrite(t, filepath.Join(personal, "skills", "s", "SKILL.md"), "x")
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Dir); !os.IsNotExist(err) {
		t.Errorf("profile dir still there: %v", err)
	}
	for _, kept := range []string{"config.toml", filepath.Join("plugins", "cache", "m", "p", "file"), filepath.Join("skills", "s", "SKILL.md")} {
		if _, err := os.Stat(filepath.Join(personal, kept)); err != nil {
			t.Errorf("deleting the profile took Personal's %s: %v", kept, err)
		}
	}
	if _, err := ConfigDir(p.Slug); !errors.Is(err, ErrProfileGone) {
		t.Errorf("ConfigDir of a deleted profile: %v, want ErrProfileGone", err)
	}
	if err := Delete(Personal); err == nil {
		t.Error("Personal was deleted")
	}
}

func TestDirRefusesAnythingButASlug(t *testing.T) {
	machine(t)
	for _, slug := range []string{"../x", "a/b", "", "Upper", Personal, "-x"} {
		if _, err := Dir(slug); err == nil {
			t.Errorf("Dir(%q) accepted", slug)
		}
	}
}

func TestEnsureSharedLinksWhatPersonalGainedLater(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(personal, "AGENTS.md"), "rules")
	mustWrite(t, filepath.Join(p.Dir, "config.toml"), "model = \"mine\"\n\n[plugins.\"own@m\"]\nenabled = true\n")
	mustWrite(t, filepath.Join(personal, "config.toml"), "model = \"shared\"\n")
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "AGENTS.md")) {
		t.Error("AGENTS.md created after the profile was not linked")
	}
	want := "model = \"shared\"\n\n[plugins.\"own@m\"]\nenabled = true\n"
	if data, _ := os.ReadFile(filepath.Join(p.Dir, "config.toml")); string(data) != want {
		t.Errorf("config.toml after a spawn = %q, want Personal's settings and the profile's plugins %q", data, want)
	}
	if err := EnsureShared(Personal); err != nil {
		t.Errorf("Personal: %v", err)
	}
	if err := EnsureShared("gone"); !errors.Is(err, ErrProfileGone) {
		t.Errorf("missing profile: %v", err)
	}
}

func TestEnvHandsTheProfileItsOwnLogin(t *testing.T) {
	got := Env([]string{
		"PATH=/bin", "CODEX_API_KEY=k", "CODEX_ACCESS_TOKEN=a", "OPENAI_API_KEY=o",
		"CODEX_SQLITE_HOME=/db", "CODEX_HOME=/old",
	}, "/new")
	if want := []string{"PATH=/bin", "CODEX_HOME=/new"}; !slices.Equal(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

func TestPersonalFollowsLichsEnvironment(t *testing.T) {
	machine(t)
	custom := t.TempDir()
	t.Setenv(HomeVar, custom)
	dir, err := ConfigDir("")
	if err != nil || dir != custom {
		t.Errorf("ConfigDir(\"\") = %q, %v, want %q", dir, err, custom)
	}
}

func TestDevRigKeepsItsOwnProfiles(t *testing.T) {
	machine(t)
	real, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LICH_DEV", "1")
	dev, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	if real == dev {
		t.Errorf("a dev rig shares %s with the installed lich", dev)
	}
}
