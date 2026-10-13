package ompprofile

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/profilestore"
)

// machine points every directory this package resolves at a scratch home and
// returns Personal's agent directory.
func machine(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("LICH_DEV", "")
	t.Setenv(HomeVar, "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	personal := filepath.Join(home, ".omp", "agent")
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

func TestCreateLaysTheProfileOutUnderOmpsOwnProfiles(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "config.yml"), "theme: {dark: nord}\n")
	mustWrite(t, filepath.Join(personal, "mcp.json"), "{}")
	mustWrite(t, filepath.Join(personal, "extensions", "lich.js"), "//")
	mustWrite(t, filepath.Join(personal, "agent.db"), "login")
	mustWrite(t, filepath.Join(personal, "sessions", "-w", "1_a.jsonl"), "{}")

	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Dir(filepath.Dir(personal))
	if want := filepath.Join(home, ".omp", "profiles", "work", "agent"); p.Dir != want {
		t.Fatalf("Dir = %s, want %s", p.Dir, want)
	}
	for _, name := range []string{"config.yml", "mcp.json", "extensions"} {
		if !profilestore.IsLink(filepath.Join(p.Dir, name)) {
			t.Errorf("%s is not linked to Personal's", name)
		}
	}
	for _, name := range []string{"agent.db", "sessions"} {
		if _, err := os.Lstat(filepath.Join(p.Dir, name)); err == nil {
			t.Errorf("%s is in the profile: its login and conversations are its own", name)
		}
	}
	if _, err := os.Stat(filepath.Join(personal, "agent.db")); err != nil {
		t.Errorf("Personal lost its login: %v", err)
	}
	got, err := ConfigDir("work")
	if err != nil || got != p.Dir {
		t.Errorf("ConfigDir = %q, %v", got, err)
	}
}

func TestDefaultIsReservedBecauseOmpReadsItAsPersonal(t *testing.T) {
	machine(t)
	p, err := Create("Default")
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug == "default" {
		t.Errorf("a profile took omp's name for the default profile: %q", p.Slug)
	}
	if _, err := Dir("default"); err == nil {
		t.Error("Dir accepted default")
	}
}

func TestASlugAlreadyAnOmpProfileIsTaken(t *testing.T) {
	personal := machine(t)
	mustMkdir(t, filepath.Join(filepath.Dir(filepath.Dir(personal)), ".omp", "profiles", "work", "agent"))
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "work-2" {
		t.Errorf("a profile omp already had was taken over: %q", p.Slug)
	}
}

func TestDeleteTakesTheWholeProfileAndNeverPersonal(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "skills", "s", "SKILL.md"), "x")
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Dir(p.Dir)
	mustWrite(t, filepath.Join(root, "logs", "omp.log"), "x")

	if err := Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("the profile's directory is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(personal, "skills", "s", "SKILL.md")); err != nil {
		t.Errorf("Personal lost a skill: %v", err)
	}
	if _, err := ConfigDir(p.Slug); !errors.Is(err, ErrProfileGone) {
		t.Errorf("ConfigDir of a deleted profile: %v", err)
	}
	if err := Delete(Personal); err == nil {
		t.Error("deleted Personal")
	}
}

func TestEnvNamesTheProfileAndDropsTheBroker(t *testing.T) {
	machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	got := Env([]string{"A=1", "OMP_AUTH_BROKER_URL=https://b", "OMP_AUTH_BROKER_TOKEN=t", "OMP_PROFILE=other"}, p.Dir)
	want := []string{"A=1", "OMP_PROFILE=work"}
	if !slices.Equal(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

func TestPersonalFollowsLichsEnvironment(t *testing.T) {
	personal := machine(t)
	home := filepath.Dir(filepath.Dir(personal))
	if got, _ := PersonalDir(); got != personal {
		t.Errorf("PersonalDir = %q, want %q", got, personal)
	}
	t.Setenv("PI_CODING_AGENT_DIR", "/custom/agent")
	if got, _ := PersonalDir(); got != "/custom/agent" {
		t.Errorf("with an override: %q", got)
	}
	t.Setenv(HomeVar, "daily")
	want := filepath.Join(home, ".omp", "profiles", "daily", "agent")
	if got, _ := PersonalDir(); got != want {
		t.Errorf("a named profile wins over the override: %q, want %q", got, want)
	}
}

func TestAnExtensionPersonalGainedLaterIsLinkedOnTheNextSpawn(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(p.Dir, "extensions")); err == nil {
		t.Fatal("linked an extensions directory Personal did not have")
	}
	mustWrite(t, filepath.Join(personal, "extensions", "lich.js"), "//")
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "extensions")) {
		t.Error("lich's extension is not in the profile after the next spawn")
	}
}

func TestRenameKeepsTheAgentDirectory(t *testing.T) {
	machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := Rename(p.Slug, "Day job"); err != nil {
		t.Fatal(err)
	}
	list, _ := List()
	if len(list) != 2 || list[1].Name != "Day job" || list[1].Dir != p.Dir || len(ConfigDirs()) != 2 {
		t.Errorf("List = %+v", list)
	}
	if root, err := Root(); err != nil || filepath.Base(root) != "omp" {
		t.Errorf("Root = %q, %v", root, err)
	}
}
