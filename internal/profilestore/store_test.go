package profilestore

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// toy is a provider that links one file, one directory and one download, and
// seeds a marker file.
func toy(personal string) Spec {
	return Spec{
		Provider:    "toy",
		Label:       "Toy",
		HomeVar:     "TOY_HOME",
		PersonalDir: func() (string, error) { return personal, nil },
		SharedFiles: []string{"settings.json"},
		Shared:      []string{"skills"},
		Downloads:   []string{filepath.Join("plugins", "cache")},
		Override:    []string{"TOY_TOKEN"},
		Reserved:    []string{"default"},
		Seed: func(_, dir string) error {
			return WriteFileAtomic(filepath.Join(dir, "seeded"), []byte("x"), 0o600)
		},
	}
}

// machine points the user's config dir (XDG on Linux, APPDATA on Windows, the
// home on macOS) at a scratch home and returns Personal's directory under it.
func machine(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("LICH_DEV", "")
	personal := filepath.Join(home, ".toy")
	mustWrite(t, filepath.Join(personal, "settings.json"), "{}")
	return personal
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

func TestCreateLaysTheProfileOutAgainstPersonal(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "skills", "s", "SKILL.md"), "x")
	s := toy(personal)

	p, err := s.Create("Work Account")
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "work-account" || p.Name != "Work Account" {
		t.Errorf("profile = %+v", p)
	}
	for _, name := range []string{"settings.json", "skills", filepath.Join("plugins", "cache")} {
		if !IsLink(filepath.Join(p.Dir, name)) {
			t.Errorf("%s is not a link", name)
		}
	}
	if _, err := os.Stat(filepath.Join(personal, "plugins", "cache")); err != nil {
		t.Errorf("the download was not made in Personal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "seeded")); err != nil {
		t.Errorf("seed did not run: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || !list[0].Personal || list[0].Dir != personal || list[1].Slug != p.Slug {
		t.Errorf("List = %+v", list)
	}
}

func TestSlugsAreFreeAndNeverPersonalOrReserved(t *testing.T) {
	s := toy(machine(t))
	long := strings.Repeat("a", 50)
	for _, c := range []struct{ name, want string }{
		{"Work Account", "work-account"},
		{"Work Account", "work-account-2"},
		{"Personal", "personal-2"},
		{"Default", "default-2"},
		{"!!!", "profile"},
		{long, strings.Repeat("a", 32)},
		{long, strings.Repeat("a", 32) + "-2"},
		{"a  b---c", "a-b-c"},
		{"Ünï", "n"},
	} {
		p, err := s.Create(c.name)
		if err != nil {
			t.Fatalf("Create(%q): %v", c.name, err)
		}
		if p.Slug != c.want {
			t.Errorf("Create(%q).Slug = %q, want %q", c.name, p.Slug, c.want)
		}
	}
	if _, err := s.Create("  "); err == nil {
		t.Error("Create accepted a blank name")
	}
}

func TestDirRefusesAnythingButASlug(t *testing.T) {
	s := toy(machine(t))
	for _, slug := range []string{"", "personal", "../x", "a/b", "A", "-a", strings.Repeat("a", 41)} {
		if _, err := s.Dir(slug); err == nil {
			t.Errorf("Dir(%q) accepted", slug)
		}
	}
}

func TestRenameKeepsTheDirectoryAndPersonalHasItsOwnName(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(p.Slug, "Day job"); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(Personal, "Home"); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(p.Slug, " "); err == nil {
		t.Error("renamed to a blank name")
	}
	list, _ := s.List()
	if list[0].Name != "Home" || list[1].Name != "Day job" || list[1].Dir != p.Dir {
		t.Errorf("List = %+v", list)
	}
	entries, _ := os.ReadDir(personal)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") && e.Name() != "settings.json" {
			t.Errorf("a sidecar landed beside Personal's directory: %s", e.Name())
		}
	}
}

func TestDeleteNeverReachesPersonal(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "skills", "s", "SKILL.md"), "x")
	s := toy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"settings.json", filepath.Join("skills", "s", "SKILL.md")} {
		if _, err := os.Stat(filepath.Join(personal, name)); err != nil {
			t.Errorf("Personal lost %s: %v", name, err)
		}
	}
	if _, err := s.ConfigDir(p.Slug); !errors.Is(err, ErrGone) {
		t.Errorf("ConfigDir of a deleted profile: %v, want ErrGone", err)
	}
	if err := s.Delete(Personal); err == nil {
		t.Error("deleted Personal")
	}
}

func TestEnsureSharedLinksWhatPersonalGainedLater(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(p.Dir, "skills")); err == nil {
		t.Fatal("linked a directory Personal did not have")
	}
	mustWrite(t, filepath.Join(personal, "skills", "s"), "x")
	if err := s.EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if !IsLink(filepath.Join(p.Dir, "skills")) {
		t.Error("the new directory was not linked")
	}
	if err := s.EnsureShared("gone"); !errors.Is(err, ErrGone) {
		t.Errorf("EnsureShared of a profile that does not exist: %v", err)
	}
	if err := s.EnsureShared(Personal); err != nil {
		t.Errorf("EnsureShared(Personal): %v", err)
	}
}

func TestEnvHandsTheProfileItsOwnLogin(t *testing.T) {
	s := toy(machine(t))
	got := s.Env([]string{"A=1", "TOY_TOKEN=secret", "TOY_HOME=/old", "B=2"}, "/profile")
	want := []string{"A=1", "B=2", "TOY_HOME=/profile"}
	if !slices.Equal(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

func TestEnvPointsPinnedVariablesUnderTheProfile(t *testing.T) {
	s := toy(machine(t))
	s.Override = append(s.Override, "TOY_STATE")
	s.Pinned = []PinnedVar{{Name: "TOY_STATE", Rel: "Toy"}}
	profile := filepath.Join(t.TempDir(), "work")
	got := s.Env([]string{"A=1", "TOY_STATE=/elsewhere"}, profile)
	want := []string{"A=1", "TOY_HOME=" + profile, "TOY_STATE=" + filepath.Join(profile, "Toy")}
	if !slices.Equal(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

func TestADirOfPlacesTheStateElsewhereThanLichsRoot(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	state := filepath.Join(t.TempDir(), "profiles")
	s.DirOf = func(slug string) (string, error) { return filepath.Join(state, slug, "agent"), nil }

	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(state, "work", "agent"); p.Dir != want {
		t.Errorf("Dir = %s, want %s", p.Dir, want)
	}
	if got, err := s.ConfigDir("work"); err != nil || got != p.Dir {
		t.Errorf("ConfigDir = %q, %v", got, err)
	}
	if p2, _ := s.Create("Work"); p2.Slug != "work-2" {
		t.Errorf("a second Work took %q, want work-2", p2.Slug)
	}
	// A profile directory that exists without a sidecar is the provider's own:
	// its slug is taken.
	if err := os.MkdirAll(filepath.Join(state, "mine", "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if p3, _ := s.Create("Mine"); p3.Slug != "mine-2" {
		t.Errorf("Mine took %q over a directory that exists, want mine-2", p3.Slug)
	}
}

func TestMoveConversationRollsBackAHalfMove(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	locate := func(dir, id string) (string, bool) {
		path := filepath.Join(dir, id+".jsonl")
		_, err := os.Stat(path)
		return path, err == nil
	}
	entries := func(id, transcript, fromDir, toDir string) ([]Pair, error) {
		return []Pair{
			{From: transcript, To: filepath.Join(toDir, filepath.Base(transcript))},
			{From: filepath.Join(fromDir, "missing"), To: filepath.Join(toDir, "missing")},
		}, nil
	}
	mustWrite(t, filepath.Join(personal, "c1.jsonl"), "x")
	if err := s.MoveConversation("c1", p.Slug, locate, entries); err == nil {
		t.Fatal("a move of an entry that does not exist succeeded")
	}
	if _, err := os.Stat(filepath.Join(personal, "c1.jsonl")); err != nil {
		t.Errorf("the failed move left the transcript behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "c1.jsonl")); !os.IsNotExist(err) {
		t.Error("the failed move left half the conversation in the target")
	}
	if err := s.MoveConversation("c2", p.Slug, locate, entries); err == nil ||
		!strings.Contains(err.Error(), `no Toy conversation "c2"`) {
		t.Errorf("a conversation no profile holds: %v", err)
	}
}

func TestMoveCopiesAcrossDirectoriesItCannotRename(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a")
	mustWrite(t, filepath.Join(src, "sub", "f"), "data")
	dst := filepath.Join(t.TempDir(), "b")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dst, "sub", "f")); err != nil || string(data) != "data" {
		t.Errorf("copied %q, %v", data, err)
	}
}

func TestADevRigKeepsItsOwnSidecars(t *testing.T) {
	s := toy(machine(t))
	real, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LICH_DEV", "1")
	dev, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	if real == dev || filepath.Base(filepath.Dir(dev)) != "profiles-dev" || filepath.Base(dev) != "toy" {
		t.Errorf("real %s, dev %s", real, dev)
	}
}

func TestSyncRunsAtCreationAndBeforeEverySpawn(t *testing.T) {
	personal := machine(t)
	s := toy(personal)
	runs := 0
	s.Sync = func(from, dir string) error {
		runs++
		if from != personal {
			t.Errorf("Sync given %s, want Personal's %s", from, personal)
		}
		return WriteFileAtomic(filepath.Join(dir, "synced"), []byte("x"), 0o600)
	}
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if runs != 2 {
		t.Errorf("Sync ran %d times over a creation and a spawn, want 2", runs)
	}
	if _, err := os.Stat(filepath.Join(p.Dir, "synced")); err != nil {
		t.Errorf("Sync did not write into the profile: %v", err)
	}
	if err := s.EnsureShared(Personal); err != nil || runs != 2 {
		t.Errorf("Personal: err %v, Sync ran %d times, want it left alone", err, runs)
	}
	s.Sync = func(_, _ string) error { return errors.New("broken") }
	if err := s.EnsureShared(p.Slug); err == nil {
		t.Error("a failed Sync did not fail the spawn")
	}
}

// Two spawns into one profile write its files at the same time: each write
// must land whole, without the other's rename taking its temporary file.
func TestWriteFileAtomicSurvivesConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.json")
	const writers = 16
	errs := make(chan error, writers)
	for i := range writers {
		go func() { errs <- WriteFileAtomic(path, []byte(strconv.Itoa(i)), 0o600) }()
	}
	for range writers {
		if err := <-errs; err != nil {
			t.Errorf("concurrent write: %v", err)
		}
	}
	leftovers, err := filepath.Glob(path + "*.tmp")
	if err != nil || len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v, %v", leftovers, err)
	}
}
