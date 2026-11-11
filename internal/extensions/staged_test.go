package extensions

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// stagedDirs is the staging directories under dir.
func stagedDirs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), stagingPrefix) {
			out = append(out, e.Name())
		}
	}
	return out
}

func prepareDir(t *testing.T, s *Service, from string) api.ExtensionInstallPreview {
	t.Helper()
	preview, err := s.PrepareFromDir(context.Background(), api.ExtensionDirInstallOptions{Dir: from})
	if err != nil {
		t.Fatalf("PrepareFromDir() = %v", err)
	}
	return preview
}

func TestPrepareStagesWithoutRunning(t *testing.T) {
	from := install(t, t.TempDir(), testID, modeEcho)
	dir := filepath.Join(t.TempDir(), "extensions")
	s, r := boot(t, dir)
	prepareDir(t, s, from)
	if got := stagedDirs(t, dir); len(got) != 1 {
		t.Fatalf("staged = %v, want one staged copy", got)
	}
	if _, err := os.Lstat(filepath.Join(dir, testID)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the extension is in place before the commit: %v", err)
	}
	if got := r.lastStatus(); got.ID != "" {
		t.Fatalf("status %+v before the commit, want nothing started", got)
	}
}

func TestDiscardDeletesTheStagedCopy(t *testing.T) {
	from := install(t, t.TempDir(), testID, modeEcho)
	dir := filepath.Join(t.TempDir(), "extensions")
	s, _ := boot(t, dir)
	preview := prepareDir(t, s, from)
	if _, err := s.DiscardInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token}); err != nil {
		t.Fatalf("DiscardInstall() = %v", err)
	}
	assertNothingInstalled(t, s, dir)
	if _, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token}); !errors.Is(err, errNoStaged) {
		t.Fatalf("CommitInstall() after a discard = %v, want the token refused", err)
	}
}

func TestATokenCommitsOnce(t *testing.T) {
	from := install(t, t.TempDir(), testID, modeEcho)
	dir := filepath.Join(t.TempDir(), "extensions")
	s, r := boot(t, dir)
	preview := prepareDir(t, s, from)
	commit(t, s, preview)
	waitStatus(t, r, statusRunning)
	if got := stagedDirs(t, dir); len(got) != 0 {
		t.Fatalf("staged = %v after the commit, want the work directory gone", got)
	}
	for _, call := range []func() error{
		func() error {
			_, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token})
			return err
		},
		func() error {
			_, err := s.DiscardInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token})
			return err
		},
		func() error {
			_, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: "unknown"})
			return err
		},
	} {
		if err := call(); !errors.Is(err, errNoStaged) {
			t.Fatalf("a reused or unknown token = %v, want %v", err, errNoStaged)
		}
	}
	if list, _ := s.List(context.Background(), api.None{}); len(list) != 1 {
		t.Fatalf("List() = %+v, want the one install", list)
	}
}

func TestASecondStagedCopyOfAnIDCommittedSinceIsRefused(t *testing.T) {
	from := install(t, t.TempDir(), testID, modeEcho)
	dir := filepath.Join(t.TempDir(), "extensions")
	s, _ := boot(t, dir)
	first, second := prepareDir(t, s, from), prepareDir(t, s, from)
	commit(t, s, first)
	_, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: second.Token})
	if err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Fatalf("CommitInstall(second) = %v, want the already-installed refusal", err)
	}
	if got := stagedDirs(t, dir); len(got) != 0 {
		t.Fatalf("staged = %v, want the refused copy deleted", got)
	}
}

func TestAStagedCopyNobodyCommitsExpires(t *testing.T) {
	const ttl = 20 * time.Millisecond
	from := install(t, t.TempDir(), testID, modeEcho)
	dir := filepath.Join(t.TempDir(), "extensions")
	s, _ := boot(t, dir, func(tm *timing) { tm.stagedTTL = ttl })
	preview := prepareDir(t, s, from)
	waitUntil(t, "the staged copy expired", func() bool { return len(stagedDirs(t, dir)) == 0 })
	if _, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token}); !errors.Is(err, errNoStaged) {
		t.Fatalf("CommitInstall() after the expiry = %v, want the token refused", err)
	}
	assertNothingInstalled(t, s, dir)
}

func TestBootDeletesWhatAnEarlierRunLeftStaged(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, stagingPrefix+"123", testID)
	install(t, filepath.Dir(left), testID, modeEcho)
	install(t, dir, "kept", modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, statusRunning)
	if got := stagedDirs(t, dir); len(got) != 0 {
		t.Fatalf("staged = %v after boot, want them deleted", got)
	}
	if list, _ := s.List(context.Background(), api.None{}); len(list) != 1 || list[0].ID != "kept" {
		t.Fatalf("List() = %+v, want the installed extension alone", list)
	}
}
