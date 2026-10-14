//go:build !windows

package profilestore

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// xdgToy is a provider whose profile is a data home of links: its own folder
// holds a login database the profile keeps and a bundle every profile shares.
func xdgToy(personal string) Spec {
	return Spec{
		Provider:    "xdgtoy",
		Label:       "XDG Toy",
		HomeVar:     "XDG_DATA_HOME",
		PersonalDir: func() (string, error) { return personal, nil },
		Own:         []string{filepath.Join("toy", "login.db"), "whole"},
	}
}

// dataHome is a Personal data home holding another tool's directory, the
// provider's folder and a folder the provider owns outright.
func dataHome(t *testing.T) string {
	t.Helper()
	personal := filepath.Join(machine(t), "..", ".local", "share")
	mustWrite(t, filepath.Join(personal, "mise", "installs"), "x")
	mustWrite(t, filepath.Join(personal, "notes.txt"), "x")
	mustWrite(t, filepath.Join(personal, "toy", "login.db"), "personal login")
	mustWrite(t, filepath.Join(personal, "toy", "bundle.js"), "x")
	mustWrite(t, filepath.Join(personal, "whole", "db"), "personal")
	return personal
}

func TestALinkHomeLinksEverythingButWhatTheProfileOwns(t *testing.T) {
	personal := dataHome(t)
	s := xdgToy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mise", "notes.txt", filepath.Join("toy", "bundle.js")} {
		if !IsLink(filepath.Join(p.Dir, name)) {
			t.Errorf("%s is not a link", name)
		}
	}
	if info, err := os.Lstat(filepath.Join(p.Dir, "toy")); err != nil || !info.IsDir() {
		t.Errorf("toy is not a directory of the profile's own: %v", err)
	}
	for _, name := range []string{filepath.Join("toy", "login.db"), "whole"} {
		if _, err := os.Lstat(filepath.Join(p.Dir, name)); err == nil {
			t.Errorf("%s exists in a new profile; it is the profile's own to make", name)
		}
	}
	if got := s.Env(nil, p.Dir); len(got) != 1 || got[0] != "XDG_DATA_HOME="+p.Dir {
		t.Errorf("Env = %v", got)
	}
}

func TestEnsureSharedKeepsTheLinkHomeInStepWithPersonal(t *testing.T) {
	personal := dataHome(t)
	s := xdgToy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(personal, "gh", "hosts.yml"), "x")
	if err := os.RemoveAll(filepath.Join(personal, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(p.Dir, "newtool", "state"), "written in the profile")
	mustWrite(t, filepath.Join(p.Dir, "toy", "fresh.cache"), "written in the profile")
	mustWrite(t, filepath.Join(p.Dir, "toy", "login.db"), "work login")
	mustWrite(t, filepath.Join(p.Dir, "whole", "db"), "work")

	if err := s.EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if !IsLink(filepath.Join(p.Dir, "gh")) {
		t.Error("a directory Personal gained was not linked")
	}
	if _, err := os.Lstat(filepath.Join(p.Dir, "notes.txt")); err == nil {
		t.Error("a link to an entry Personal lost was kept, so no tool can make it again")
	}
	for _, name := range []string{"newtool", filepath.Join("toy", "fresh.cache")} {
		if IsLink(filepath.Join(p.Dir, name)) {
			t.Errorf("%s, made under the profile, became a link", name)
		}
		if _, err := os.Lstat(filepath.Join(personal, name)); err == nil {
			t.Errorf("%s, made under the profile, was written into Personal", name)
		}
	}
	for name, want := range map[string]string{filepath.Join("toy", "login.db"): "personal login", filepath.Join("whole", "db"): "personal"} {
		if data, _ := os.ReadFile(filepath.Join(personal, name)); string(data) != want {
			t.Errorf("Personal's %s = %q, want %q: the profile's own entry reached it", name, data, want)
		}
		if IsLink(filepath.Join(p.Dir, name)) {
			t.Errorf("the profile's own %s became a link", name)
		}
	}
}

func TestALinkHomeNeverWritesIntoPersonalThroughALink(t *testing.T) {
	personal := dataHome(t)
	s := xdgToy(personal)
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	// A provider folder that is a link (made by hand, or by an older layout)
	// is Personal's: walking it would lay links out inside Personal's folder.
	if err := os.RemoveAll(filepath.Join(p.Dir, "toy")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(personal, "toy"), filepath.Join(p.Dir, "toy")); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(personal, "toy"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			t.Errorf("a link %s appeared in Personal's folder", e.Name())
		}
	}
	if err := s.Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{filepath.Join("mise", "installs"), filepath.Join("toy", "login.db"), filepath.Join("whole", "db")} {
		if _, err := os.Stat(filepath.Join(personal, name)); err != nil {
			t.Errorf("deleting the profile took Personal's %s: %v", name, err)
		}
	}
}

// TestALinkHomeHoldsNoPathBackToTheProfiles: when Personal's base directory is
// the one lich's config dir lives in (Cursor CLI's XDG_CONFIG_HOME), the entry
// holding the profiles is never linked, at creation or after, so nothing in a
// link home leads back into it.
func TestALinkHomeHoldsNoPathBackToTheProfiles(t *testing.T) {
	config := filepath.Join(machine(t), "..", ".config")
	mustWrite(t, filepath.Join(config, "gh", "hosts.yml"), "x")
	s := xdgToy(config)
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	// A link an older layout left behind is pruned at the next spawn.
	if _, err := os.Lstat(filepath.Join(p.Dir, "lich")); err == nil {
		t.Error("the directory holding the profiles was linked at creation")
	} else if err := os.Symlink(filepath.Join(config, "lich"), filepath.Join(p.Dir, "lich")); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if !IsLink(filepath.Join(p.Dir, "gh")) {
		t.Error("gh was not linked")
	}
	err = filepath.WalkDir(p.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.Type()&fs.ModeSymlink == 0 {
			return err
		}
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		target = filepath.Clean(target)
		if target == root || strings.HasPrefix(root, target+string(filepath.Separator)) || strings.HasPrefix(target, root) {
			t.Errorf("%s leads back to the profiles at %s", path, target)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
