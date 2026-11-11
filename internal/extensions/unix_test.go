//go:build !windows

package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func TestSymlinkOutOfTheDirectoryIsInvalid(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "elsewhere")
	writeFile(t, outside, "#!/bin/sh\n")
	root := install(t, dir, "ext", modeEcho, func(m *extension.Manifest) { m.Executable = "link" })
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	found := discover(dir, "1.2.3")
	if len(found) != 1 || !strings.Contains(found[0].errText, "resolves outside") {
		t.Fatalf("discover = %+v", found[0])
	}
}

func TestSymlinkInsideTheDirectoryIsFollowed(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeEcho, func(m *extension.Manifest) { m.Executable = "link" })
	entries, err := filepath.Glob(filepath.Join(root, "ext*"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("fixture: %v, %v", entries, err)
	}
	if err := os.Symlink(entries[0], filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if found := discover(dir, "1.2.3"); found[0].invalid != nil {
		t.Errorf("rejected: %v", found[0].invalid)
	}
}

func TestSymlinkedExtensionDirectoryIsListed(t *testing.T) {
	source := t.TempDir()
	root := install(t, source, "ext", modeEcho)
	dir := t.TempDir()
	if err := os.Symlink(root, filepath.Join(dir, "ext")); err != nil {
		t.Fatal(err)
	}
	if found := discover(dir, "1.2.3"); len(found) != 1 || found[0].invalid != nil {
		t.Errorf("discover = %+v", found)
	}
}

func TestDanglingSymlinkIsListedInvalid(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "ext")); err != nil {
		t.Fatal(err)
	}
	if found := discover(dir, "1.2.3"); len(found) != 1 || found[0].invalid == nil {
		t.Errorf("discover = %+v", found)
	}
}

// A file that cannot be executed stays that way, so one try is enough.
func TestAFileThatCannotStartFailsWithoutRetrying(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, "ext", modeEcho, func(m *extension.Manifest) { m.Executable = "plain" })
	writeFile(t, filepath.Join(root, "plain"), "not a program")
	_, r := boot(t, dir)

	waitStatus(t, r, api.ExtensionFailed)
	if last := r.lastStatus(); !strings.Contains(last.Error, "could not start") {
		t.Errorf("error = %q", last.Error)
	}
	if got := r.statusNames(); len(got) != 1 {
		t.Errorf("statuses = %v, want only failed", got)
	}
}
