package extensions

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

const savedFile = "token"

func TestTheProcessGetsItsDataDirectoryCreated(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, testID, modeDataDir)
	s, r := boot(t, dir)
	want := filepath.Join(s.dataDir, testID)
	waitUntil(t, "the data directory reported", func() bool { return slices.Contains(r.titles(t), want) })
	info, err := os.Stat(want)
	if err != nil || !info.IsDir() {
		t.Fatalf("the data directory = %v, %v, want a directory", info, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		t.Errorf("the data directory's mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestRemoveKeepsTheDataDirectory(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, testID, modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, statusRunning)
	saved := filepath.Join(s.dataDir, testID, savedFile)
	writeFile(t, saved, "secret")

	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if raw, err := os.ReadFile(saved); err != nil || string(raw) != "secret" {
		t.Fatalf("what the extension saved = %q, %v after Remove, want it kept", raw, err)
	}
}

func TestDeleteDataDeletesTheDataOfARemovedExtension(t *testing.T) {
	dir := t.TempDir()
	install(t, dir, testID, modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, statusRunning)
	data := filepath.Join(s.dataDir, testID)
	writeFile(t, filepath.Join(data, savedFile), "secret")

	if _, err := s.DeleteData(context.Background(), api.ExtensionIDOptions{ID: testID}); err == nil ||
		!strings.Contains(err.Error(), "remove it first") {
		t.Fatalf("DeleteData() of an installed extension = %v, want it refused", err)
	}
	if _, err := os.Stat(data); err != nil {
		t.Fatalf("a refused DeleteData() touched the data: %v", err)
	}
	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, err := s.DeleteData(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("DeleteData() = %v", err)
	}
	if _, err := os.Lstat(data); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the data directory is still there: %v", err)
	}
}

func TestDeleteDataRefusesAnIDThatLeavesTheDirectory(t *testing.T) {
	s, _ := boot(t, t.TempDir())
	outside := filepath.Join(filepath.Dir(s.dataDir), "outside")
	writeFile(t, outside, "kept")
	for _, id := range []string{"", "..", "../outside", "a/b", `a\b`, "/abs"} {
		if _, err := s.DeleteData(context.Background(), api.ExtensionIDOptions{ID: id}); err == nil {
			t.Errorf("DeleteData(%q) = nil, want it refused", id)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("a file beside the data directory went: %v", err)
	}
}
