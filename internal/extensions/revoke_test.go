package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

const (
	revokedReasonText = "it uploads your files"
	revokedText       = "removed from the lich catalog: " + revokedReasonText
)

func revokeTestRepo(t *testing.T, s *Service) {
	t.Helper()
	f := listAs(t, s)
	cat := catalogOf()
	cat.Revoked = []extension.Revocation{{Repo: "https://github.com/SOMEONE/fake-ext", Reason: revokedReasonText}}
	f.publish(t, cat)
	s.refreshCatalog()
}

// revokedInstall is testID installed from GitHub, running, then revoked.
func revokedInstall(t *testing.T) (*Service, *recorder, string) {
	t.Helper()
	s, r, dir := installedFromGit(t)
	revokeTestRepo(t, s)
	return s, r, dir
}

func TestARevokedExtensionIsStoppedAndListedWithTheReason(t *testing.T) {
	s, r, dir := revokedInstall(t)
	got := only(t, s)
	if got.Status != statusInvalid || got.Error != revokedText {
		t.Fatalf("extension = %s %q, want invalid with %q", got.Status, got.Error, revokedText)
	}
	if last := r.lastStatus(); last.Status != statusInvalid || last.Error != revokedText {
		t.Errorf("window last told %s %q, want the revocation", last.Status, last.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, testID, eofFile)); err != nil {
		t.Errorf("the extension was not stopped through its stdin: %v", err)
	}
	if src := readSource(filepath.Join(dir, testID)); src.Revoked != revokedReasonText {
		t.Errorf("source record = %+v, want the reason kept", src)
	}
	if _, err := s.Restart(context.Background(), api.ExtensionIDOptions{ID: testID}); err == nil {
		t.Error("Restart started a revoked extension")
	}
}

func TestARevokedExtensionStaysStoppedWithNoCatalog(t *testing.T) {
	s, _, dir := revokedInstall(t)
	runs := runCount(filepath.Join(dir, testID))
	s.Shutdown()
	again, _ := boot(t, dir)
	if got := only(t, again); got.Status != statusInvalid || got.Error != revokedText {
		t.Fatalf("after a restart = %s %q, want invalid with %q", got.Status, got.Error, revokedText)
	}
	if runCount(filepath.Join(dir, testID)) != runs {
		t.Error("a revoked extension ran after a restart")
	}
	_, err := again.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err == nil || !strings.Contains(err.Error(), revokedText) {
		t.Errorf("PrepareUpdate() = %v, want it refused with the reason", err)
	}
}

func TestARevokedExtensionIsRemovedKeepingItsData(t *testing.T) {
	s, r, dir := revokedInstall(t)
	data := filepath.Join(s.dataDir, testID)
	if _, err := os.Stat(data); err != nil {
		t.Fatalf("no data directory to keep: %v", err)
	}
	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, testID)); !os.IsNotExist(err) {
		t.Errorf("the extension's directory is still there: %v", err)
	}
	if _, err := os.Stat(data); err != nil {
		t.Errorf("Remove deleted the data directory: %v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.removed) != 1 {
		t.Errorf("removed events = %v, want one", r.removed)
	}
}

func TestARevokedExtensionIsOfferedNoUpdate(t *testing.T) {
	s, r, _ := revokedInstall(t)
	publishNewer(t, s, newerArchive(t, testID))
	if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates() = %v", err)
	}
	if got := r.updatesSeen(); len(got) != 0 {
		t.Errorf("updates = %+v for a revoked extension, want none", got)
	}
	_, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err == nil || !strings.Contains(err.Error(), revokedText) {
		t.Errorf("PrepareUpdate() = %v, want it refused with the reason", err)
	}
}

func TestPreparingAnInstallOfARevokedRepositoryIsRefused(t *testing.T) {
	_, _, s, _, dir := twoReleases(t)
	revokeTestRepo(t, s)
	_, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err == nil || !strings.Contains(err.Error(), revokedText) {
		t.Fatalf("PrepareFromGit() = %v, want it refused with the reason", err)
	}
	assertNothingInstalled(t, s, dir)
}

func TestAnInstallRevokedWhileItWaitsIsNeverStarted(t *testing.T) {
	_, _, s, _, dir := twoReleases(t)
	preview, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err != nil {
		t.Fatalf("PrepareFromGit() = %v", err)
	}
	revokeTestRepo(t, s)
	if got := commit(t, s, preview); got.Status != statusInvalid || got.Error != revokedText {
		t.Fatalf("committed = %s %q, want invalid with %q", got.Status, got.Error, revokedText)
	}
	if runCount(filepath.Join(dir, testID)) != 0 {
		t.Error("a revoked extension ran")
	}
}

func TestAnUpdateRevokedWhileItWaitsIsNeverStarted(t *testing.T) {
	s, _, dir := installedFromGit(t)
	publishNewer(t, s, newerArchive(t, testID))
	preview, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err != nil {
		t.Fatalf("PrepareUpdate() = %v", err)
	}
	revokeTestRepo(t, s)
	got, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token})
	if err != nil {
		t.Fatalf("CommitInstall() = %v", err)
	}
	if got.Status != statusInvalid || got.Error != revokedText {
		t.Fatalf("updated = %s %q, want invalid with %q", got.Status, got.Error, revokedText)
	}
	if src := readSource(filepath.Join(dir, testID)); src.Revoked != revokedReasonText {
		t.Errorf("source record = %+v, want the reason kept by the update", src)
	}
}
