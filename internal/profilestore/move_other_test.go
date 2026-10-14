//go:build !windows

package profilestore

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCrossDeviceKnowsFilesystems(t *testing.T) {
	if !crossDevice(syscall.EXDEV) {
		t.Error("EXDEV was not read as crossing filesystems, so the copy never runs")
	}
	if crossDevice(syscall.ENOENT) {
		t.Error("a missing file was read as crossing filesystems")
	}
}

func TestCopyTreeKeepsPermissionBits(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a")
	mustWrite(t, filepath.Join(src, "hooks", "run.sh"), "#!/bin/sh")
	mustWrite(t, filepath.Join(src, "plain"), "x")
	for path, mode := range map[string]os.FileMode{
		filepath.Join(src, "hooks", "run.sh"): 0o755,
		filepath.Join(src, "plain"):           0o640,
		filepath.Join(src, "hooks"):           0o750,
	} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(t.TempDir(), "b")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]os.FileMode{
		filepath.Join("hooks", "run.sh"): 0o755,
		"plain":                          0o640,
		"hooks":                          0o750,
	} {
		info, err := os.Stat(filepath.Join(dst, rel))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s copied as %o, want %o", rel, got, want)
		}
	}
}

func TestCopyTreeRecreatesSymlinksAsSymlinks(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a")
	shared := t.TempDir()
	mustWrite(t, filepath.Join(shared, "settings.json"), "{}")
	mustWrite(t, filepath.Join(shared, "skills", "s"), "x")
	mustWrite(t, filepath.Join(src, "real"), "data")
	links := map[string]string{
		"settings.json": filepath.Join(shared, "settings.json"),
		"skills":        filepath.Join(shared, "skills"),
		"relative":      "real",
		"dangling":      filepath.Join(shared, "gone"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(src, name)); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(t.TempDir(), "b")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	for name, want := range links {
		got, err := os.Readlink(filepath.Join(dst, name))
		if err != nil || got != want {
			t.Errorf("%s: link target %q, %v, want %q", name, got, err, want)
		}
	}
}
