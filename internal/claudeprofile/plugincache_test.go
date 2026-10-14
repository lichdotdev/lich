package claudeprofile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/omartelo/lich/internal/profilestore"
)

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// installVersion writes a plugin version the way `claude plugin install` run
// in the profile at dir leaves it: a real directory under its plugins/cache and
// a record naming it.
func installVersion(t *testing.T, dir, version string) string {
	t.Helper()
	path := filepath.Join(dir, "plugins", "cache", "m", "demo", version)
	mustWrite(t, filepath.Join(path, "hooks", "run.sh"), "#!/bin/sh\n"+version)
	if err := os.Chmod(filepath.Join(path, "hooks", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	nameVersion(t, dir, version)
	return path
}

func nameVersion(t *testing.T, dir, version string) {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(dir, "plugins", "cache", "m", "demo", version))
	mustWrite(t, filepath.Join(dir, "plugins", "installed_plugins.json"),
		`{"version":2,"plugins":{"demo@m":[{"scope":"user","installPath":"`+path+`","version":"`+version+`"}]}}`)
}

func storeVersion(t *testing.T, version string) string {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, pluginStoreName, "m", "demo", version)
}

func assertLinkedToStore(t *testing.T, path, version string) {
	t.Helper()
	if !profilestore.IsLink(path) {
		t.Fatalf("%s is not a link", path)
	}
	script := filepath.Join(storeVersion(t, version), "hooks", "run.sh")
	data, err := os.ReadFile(filepath.Join(path, "hooks", "run.sh"))
	if err != nil || string(data) != "#!/bin/sh\n"+version {
		t.Fatalf("the link does not reach the version: %q, %v", data, err)
	}
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("the store lacks the version: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the stored script lost its execute bit: %v", info.Mode())
	}
}

func TestEnsureSharedLinksEachInstalledVersionIntoOneStore(t *testing.T) {
	machine(t)
	work, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	client, err := Create("Client")
	if err != nil {
		t.Fatal(err)
	}
	workVersion := installVersion(t, work.Dir, "1.0.0")
	orphan := filepath.Join(work.Dir, "plugins", "cache", "m", "demo", "0.9.0")
	mustWrite(t, filepath.Join(orphan, "file"), "x")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	assertLinkedToStore(t, workVersion, "1.0.0")
	if profilestore.IsLink(orphan) || !isDir(orphan) {
		t.Error("a version the profile does not name was moved: it is Claude Code's to sweep")
	}

	clientVersion := installVersion(t, client.Dir, "1.0.0")
	if err := EnsureShared(client.Slug); err != nil {
		t.Fatal(err)
	}
	assertLinkedToStore(t, clientVersion, "1.0.0")
	entries, err := os.ReadDir(filepath.Dir(clientVersion))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the profile's own copy of a stored version was left behind: %v", entries)
	}

	if err := EnsureShared(work.Slug); err != nil {
		t.Fatalf("a second pass: %v", err)
	}
	assertLinkedToStore(t, workVersion, "1.0.0")
}

func TestEnsureSharedLinksAStoredVersionTheProfileNamesButLacks(t *testing.T) {
	machine(t)
	work, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	client, err := Create("Client")
	if err != nil {
		t.Fatal(err)
	}
	installVersion(t, work.Dir, "1.0.0")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	nameVersion(t, client.Dir, "1.0.0")
	if err := EnsureShared(client.Slug); err != nil {
		t.Fatal(err)
	}
	assertLinkedToStore(t, filepath.Join(client.Dir, "plugins", "cache", "m", "demo", "1.0.0"), "1.0.0")
}

func TestEnsureSharedMovesACacheLinkedToPersonalsOntoItsOwn(t *testing.T) {
	personal := machine(t)
	installVersion(t, personal, "1.0.0")
	work, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(work.Dir, "plugins", "cache")
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	if err := profilestore.LinkDir(filepath.Join(personal, "plugins", "cache"), cache); err != nil {
		t.Fatal(err)
	}
	nameVersion(t, work.Dir, "1.0.0")

	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	if profilestore.IsLink(cache) || !isDir(cache) {
		t.Fatal("the profile's plugin cache still links to Personal's")
	}
	assertLinkedToStore(t, filepath.Join(cache, "m", "demo", "1.0.0"), "1.0.0")
	if _, err := os.Stat(filepath.Join(personal, "plugins", "cache", "m", "demo", "1.0.0", "hooks", "run.sh")); err != nil {
		t.Errorf("the move took Personal's own copy: %v", err)
	}
}

func TestDeletingAProfileKeepsTheStore(t *testing.T) {
	machine(t)
	work, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	installVersion(t, work.Dir, "1.0.0")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	if err := Delete(work.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(storeVersion(t, "1.0.0"), "hooks", "run.sh")); err != nil {
		t.Errorf("deleting a profile reached into the store: %v", err)
	}
}

func TestMovingOffPersonalsCacheSkipsWhatItCannotCopy(t *testing.T) {
	personal := machine(t)
	work, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(work.Dir, "plugins", "cache")
	if err := os.Remove(cache); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Join(personal, "plugins", "cache"))
	if err := profilestore.LinkDir(filepath.Join(personal, "plugins", "cache"), cache); err != nil {
		t.Fatal(err)
	}
	gone := filepath.ToSlash(filepath.Join(cache, "m", "demo", "1.0.0"))
	local := filepath.ToSlash(filepath.Join(personal, "src", "mine"))
	mustWrite(t, filepath.Join(work.Dir, "plugins", "installed_plugins.json"),
		`{"version":2,"plugins":{"demo@m":[{"installPath":"`+gone+`"}],"mine@local":[{"installPath":"`+local+`"}]}}`)

	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	if profilestore.IsLink(cache) || !isDir(cache) {
		t.Fatal("the profile's plugin cache still links to Personal's")
	}
	if _, err := os.Lstat(filepath.Join(cache, "m", "demo", "1.0.0")); !os.IsNotExist(err) {
		t.Errorf("a version Personal had lost was made up: %v", err)
	}
	if _, err := os.Lstat(storeVersion(t, "1.0.0")); !os.IsNotExist(err) {
		t.Errorf("the store holds a version nobody had: %v", err)
	}
}
