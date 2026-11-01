//go:build !windows

package providers

import (
	"context"
	"github.com/lichdotdev/lich/api"
	"os"
	"path/filepath"
	"testing"
)

// A $PATH that reaches one install twice (a directory listed twice, a symlink
// into another) is one binary, not a second one to warn about.
func TestOnPathCountsOneFileOnce(t *testing.T) {
	install, link := t.TempDir(), t.TempDir()
	script := filepath.Join(install, "codex")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'codex-cli 0.162.1'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(script, filepath.Join(link, "codex")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", install+string(os.PathListSeparator)+link+string(os.PathListSeparator)+install)

	got, err := New().OnPath(context.Background(), api.ProviderIDOptions{ID: Codex})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != script || got[0].Version != "0.162.1" {
		t.Errorf("OnPath = %+v, want the one install at %s, 0.162.1", got, script)
	}
}

func fakeVersionBin(t *testing.T, printed string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho '"+printed+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// What 2.2.5 and 1.19.4 print for --version.
func TestIsOpencode2ReadsTheVersion(t *testing.T) {
	v2, v1 := fakeVersionBin(t, "opencode v2.2.5"), fakeVersionBin(t, "1.19.4")
	if !IsOpencode2(v2) || IsOpencode2(v1) {
		t.Errorf("IsOpencode2: 2.2.5 = %v, 1.19.4 = %v", IsOpencode2(v2), IsOpencode2(v1))
	}
	if IsOpencode2(filepath.Join(t.TempDir(), "absent")) {
		t.Error("a binary that is not there read as opencode 2")
	}
}

// An upgrade in place rewrites the file, and the verdict cached for the old
// one must not outlive it.
func TestIsOpencode2SeesAnUpgradeInPlace(t *testing.T) {
	bin := fakeVersionBin(t, "1.19.4")
	if IsOpencode2(bin) {
		t.Fatal("1.19.4 read as opencode 2")
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'opencode v2.2.5' # upgraded\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsOpencode2(bin) {
		t.Error("the upgraded binary still read as 1.x")
	}
}
