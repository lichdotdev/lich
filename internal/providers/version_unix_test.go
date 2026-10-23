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
