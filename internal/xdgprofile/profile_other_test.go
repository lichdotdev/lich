//go:build !linux && !windows

package xdgprofile

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

func TestProvidersAreOpenCodeAloneOnMacOS(t *testing.T) {
	var ids []string
	for _, p := range Providers() {
		ids = append(ids, p.ID)
	}
	if !slices.Equal(ids, []string{"opencode"}) || Of("cursor") != nil || Of("kiro") != nil {
		t.Errorf("Providers = %v", ids)
	}
}

func TestAnOpenCodeProfileLinksTheDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("LICH_DEV", "")
	if dir, err := os.UserConfigDir(); err != nil || !strings.HasPrefix(dir, home) {
		t.Fatalf("lich's config dir is %s (%v), outside the scratch home", dir, err)
	}
	for _, f := range []string{"mise/installs", "opencode/opencode.db"} {
		path := filepath.Join(home, ".local", "share", filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("personal"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := OpenCode.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "mise")) {
		t.Error("mise is not linked")
	}
	if _, err := os.Lstat(filepath.Join(p.Dir, "opencode")); err == nil {
		t.Error("a new profile holds opencode's folder")
	}
	if env := OpenCode.Env(nil, p.Dir); !slices.Equal(env, []string{"XDG_DATA_HOME=" + p.Dir}) {
		t.Errorf("Env = %v", env)
	}
}
