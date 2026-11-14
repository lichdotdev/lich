package claudeprofile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// storeTwoVersions leaves Work naming 2.0.0 after an update from 1.0.0, both
// in the store and both linked from Work's cache, as a spawn after the update
// leaves them.
func storeTwoVersions(t *testing.T) Profile {
	t.Helper()
	work, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	installVersion(t, work.Dir, "1.0.0")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	installVersion(t, work.Dir, "2.0.0")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	return work
}

func versionLink(dir, version string) string {
	return filepath.Join(dir, "plugins", "cache", "m", "demo", version)
}

func assertStored(t *testing.T, version string, want bool) {
	t.Helper()
	_, err := os.Lstat(storeVersion(t, version))
	if got := err == nil; got != want {
		t.Errorf("store holds %s: %v, want %v (%v)", version, got, want, err)
	}
}

// writeMarker writes the in-use marker Claude Code leaves for a version it runs
// at dir: <pid> holding {"pid":<pid>}.
func writeMarker(t *testing.T, dir string, pid int) {
	t.Helper()
	mustWrite(t, filepath.Join(dir, strconv.Itoa(pid)), `{"pid":`+strconv.Itoa(pid)+`,"procStart":"1"}`)
}

func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestSweepDeletesAVersionNoProfileNames(t *testing.T) {
	personal := machine(t)
	work := storeTwoVersions(t)
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
	assertStored(t, "1.0.0", false)
	assertStored(t, "2.0.0", true)
	if _, err := os.Lstat(versionLink(work.Dir, "1.0.0")); !os.IsNotExist(err) {
		t.Errorf("the profile's link to the swept version is left dangling: %v", err)
	}
	assertLinkedToStore(t, versionLink(work.Dir, "2.0.0"), "2.0.0")

	nameVersion(t, personal, "2.0.0")
	installVersion(t, work.Dir, "3.0.0")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
	assertStored(t, "2.0.0", true)
}

func TestSweepKeepsAVersionAClaudeCodeMarksInUse(t *testing.T) {
	machine(t)
	work := storeTwoVersions(t)
	installVersion(t, work.Dir, "3.0.0")
	if err := EnsureShared(work.Slug); err != nil {
		t.Fatal(err)
	}
	// 2.1.325 marks a linked version in the profile's own plugins directory; an
	// older one wrote through the link into the store's copy.
	writeMarker(t, filepath.Join(work.Dir, "plugins", inUseLinksDir, "m", "demo", "1.0.0"), os.Getpid())
	writeMarker(t, filepath.Join(storeVersion(t, "2.0.0"), inUseDir), os.Getpid())
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
	assertStored(t, "1.0.0", true)
	assertStored(t, "2.0.0", true)
	assertLinkedToStore(t, versionLink(work.Dir, "1.0.0"), "1.0.0")
}

func TestSweepDeletesAVersionMarkedOnlyByAnEndedProcess(t *testing.T) {
	machine(t)
	work := storeTwoVersions(t)
	writeMarker(t, filepath.Join(work.Dir, "plugins", inUseLinksDir, "m", "demo", "1.0.0"), deadPID(t))
	writeMarker(t, filepath.Join(storeVersion(t, "1.0.0"), inUseDir), deadPID(t))
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
	assertStored(t, "1.0.0", false)
}

func TestSweepKeepsAVersionWithAMarkerItCannotRead(t *testing.T) {
	machine(t)
	work := storeTwoVersions(t)
	mustWrite(t, filepath.Join(work.Dir, "plugins", inUseLinksDir, "m", "demo", "1.0.0", "1.tmp.2"), "")
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
	assertStored(t, "1.0.0", true)
}

func TestSweepNeverFollowsALinkOutOfTheStore(t *testing.T) {
	machine(t)
	work := storeTwoVersions(t)
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "keep"), "x")
	mustMkdir(t, filepath.Join(outside, "demo", "9.9.9"))
	if err := profilestore.LinkDir(outside, filepath.Join(storeVersion(t, "1.0.0"), "escape")); err != nil {
		t.Fatal(err)
	}
	store := filepath.Dir(filepath.Dir(filepath.Dir(storeVersion(t, "1.0.0"))))
	if err := profilestore.LinkDir(outside, filepath.Join(store, "elsewhere")); err != nil {
		t.Fatal(err)
	}
	if err := profilestore.LinkDir(outside, filepath.Join(store, "m", "demo", "0.1.0")); err != nil {
		t.Fatal(err)
	}
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
	assertStored(t, "1.0.0", false)
	if _, err := os.Stat(filepath.Join(outside, "keep")); err != nil {
		t.Errorf("the sweep reached through a link out of the store: %v", err)
	}
	if !isDir(filepath.Join(outside, "demo", "9.9.9")) {
		t.Error("the sweep deleted a version reached through a linked marketplace")
	}
	assertLinkedToStore(t, versionLink(work.Dir, "2.0.0"), "2.0.0")
}

func TestSweepDeletesNothingWhenARecordCannotBeRead(t *testing.T) {
	machine(t)
	storeTwoVersions(t)
	client, err := Create("Client")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(client.Dir, "plugins", "installed_plugins.json"), "{")
	if err := SweepPluginStore(); err == nil {
		t.Error("a record that cannot be read was taken as naming nothing")
	}
	assertStored(t, "1.0.0", true)
}

func TestSweepWithoutAStore(t *testing.T) {
	machine(t)
	if _, err := Create("Work"); err != nil {
		t.Fatal(err)
	}
	if err := SweepPluginStore(); err != nil {
		t.Fatal(err)
	}
}
