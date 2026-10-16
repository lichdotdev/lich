package profilestore

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCrossDeviceKnowsWindowsVolumes(t *testing.T) {
	err := &os.LinkError{Op: "rename", Old: `C:\a`, New: `D:\b`, Err: windows.ERROR_NOT_SAME_DEVICE}
	if !crossDevice(err.Err) {
		t.Error("ERROR_NOT_SAME_DEVICE was not read as crossing volumes, so the copy never runs")
	}
}

// refuseSymlinks stands in for Windows without Developer Mode, where creating
// a symlink needs a privilege the process does not hold.
func refuseSymlinks(t *testing.T) {
	real := symlink
	symlink = func(oldname, newname string) error {
		return &os.LinkError{Op: "symlink", Old: oldname, New: newname, Err: windows.ERROR_PRIVILEGE_NOT_HELD}
	}
	t.Cleanup(func() { symlink = real })
}

func TestCopyTreeKeepsADanglingJunctionADirectoryLink(t *testing.T) {
	refuseSymlinks(t)
	src := filepath.Join(t.TempDir(), "a")
	target := filepath.Join(t.TempDir(), "shared")
	mustWrite(t, filepath.Join(target, "f"), "x")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := LinkDir(target, filepath.Join(src, "skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "b")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree over a dangling junction: %v", err)
	}
	mustWrite(t, filepath.Join(target, "f"), "back")
	got, err := os.ReadFile(filepath.Join(dst, "skills", "f"))
	if err != nil || string(got) != "back" {
		t.Errorf("read through the copied junction: %q, %v, want it to reach the target directory again", got, err)
	}
}

func TestCopyTreeLeavesOutADanglingFileLinkWindowsRefuses(t *testing.T) {
	src := filepath.Join(t.TempDir(), "a")
	mustWrite(t, filepath.Join(src, "real"), "data")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), filepath.Join(src, "dangling")); err != nil {
		t.Skipf("creating the source symlink needs Developer Mode or elevation: %v", err)
	}
	refuseSymlinks(t)
	dst := filepath.Join(t.TempDir(), "b")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree over a dangling file link: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dst, "dangling")); !os.IsNotExist(err) {
		t.Errorf("dangling link in the copy: Lstat err %v, want it left out", err)
	}
	if got, err := os.ReadFile(filepath.Join(dst, "real")); err != nil || string(got) != "data" {
		t.Errorf("real file copied as %q, %v", got, err)
	}
}

func TestLinkFileCopiesARelativeTargetFromBesideTheLink(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "real"), "data")
	refuseSymlinks(t)
	copied, err := linkFile("real", filepath.Join(dir, "link"))
	if err != nil || !copied {
		t.Fatalf("linkFile = %v, %v; want a copy", copied, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "link")); err != nil || string(got) != "data" {
		t.Errorf("copy holds %q, %v; want the file beside the link, not one under the working directory", got, err)
	}
}
