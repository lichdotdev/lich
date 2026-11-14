package extensions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hostPlatform() string { return runtime.GOOS + "_" + runtime.GOARCH }

// listedAt is the catalog's entry for testRepo at tag, spelled in another case
// than the install's URL, with the hash of each build.
func listedAt(tag, version string, builds map[string]string) extension.CatalogEntry {
	return extension.CatalogEntry{
		ID: testID, Repo: "https://github.com/SomeOne/Fake-Ext", Version: version, Tag: tag, Builds: builds,
	}
}

// listAs points s at a catalog holding entries, and reads it.
func listAs(t *testing.T, s *Service, entries ...extension.CatalogEntry) *fakeCatalog {
	t.Helper()
	f := &fakeCatalog{}
	s.catalogURL = f.serve(t)
	f.publish(t, catalogOf(entries...))
	if err := s.readCatalog(); err != nil {
		t.Fatalf("readCatalog() = %v", err)
	}
	return f
}

// twoReleases has the catalog's testTag and a newerTag GitHub calls latest.
func twoReleases(t *testing.T) (older, newer []byte, s *Service, r *recorder, dir string) {
	t.Helper()
	older = tarball(t, extensionEntries(t, testID, selfBytes(t)))
	newer = newerArchive(t, testID)
	srv := releasesServer(t, newerTag, map[string]map[string][]byte{
		testTag:  signed(map[string][]byte{hostAsset(): older}),
		newerTag: signed(map[string][]byte{hostAsset(): newer}),
	})
	s, r, dir = installer(t, srv)
	return older, newer, s, r, dir
}

func TestAListedRepositoryInstallsTheCatalogsTagNotTheLatest(t *testing.T) {
	older, _, s, r, _ := twoReleases(t)
	listAs(t, s, listedAt(testTag, "1.0.0", map[string]string{hostPlatform(): sha(older)}))
	preview, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err != nil {
		t.Fatalf("PrepareFromGit() = %v", err)
	}
	if preview.Release != testTag || preview.Version != "1.0.0" {
		t.Fatalf("preview = %+v, want the catalog's %s", preview, testTag)
	}
	commit(t, s, preview)
	waitStatus(t, r, statusRunning)
}

func TestAnUnlistedRepositoryInstallsTheLatestBesideACatalog(t *testing.T) {
	_, _, s, _, _ := twoReleases(t)
	listAs(t, s, extension.CatalogEntry{ID: "other", Repo: "https://github.com/someone/other", Version: "1.0.0", Tag: testTag})
	preview, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err != nil {
		t.Fatalf("PrepareFromGit() = %v", err)
	}
	if preview.Release != newerTag {
		t.Errorf("preview release = %s, want the latest, %s", preview.Release, newerTag)
	}
}

func TestAListedInstallRefusesWhatTheCatalogDidNotRecord(t *testing.T) {
	cases := []struct {
		name   string
		builds map[string]string
		tag    string
		want   string
	}{
		{"another archive", map[string]string{hostPlatform(): sha([]byte("the archive the catalog saw"))}, testTag, "differs from the archive the lich catalog recorded"},
		{"no build here", map[string]string{"plan9_arm": sha([]byte("x"))}, testTag, "no build of fake 1.0.0 for " + runtime.GOOS + "/" + runtime.GOARCH},
		{"a tag GitHub lacks", nil, "v0.9.0", "has no release v0.9.0, the one the lich catalog lists"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, s, _, dir := twoReleases(t)
			listAs(t, s, listedAt(c.tag, "1.0.0", c.builds))
			_, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("PrepareFromGit() = %v, want an error naming %q", err, c.want)
			}
			assertNothingInstalled(t, s, dir)
		})
	}
}

// installedListed is testID installed at testTag from a listed repository
// whose GitHub latest is newerTag.
func installedListed(t *testing.T) (s *Service, r *recorder, newer []byte) {
	t.Helper()
	older, newer, s, r, _ := twoReleases(t)
	listAs(t, s, listedAt(testTag, "1.0.0", map[string]string{hostPlatform(): sha(older)}))
	preview, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err != nil {
		t.Fatalf("PrepareFromGit() = %v", err)
	}
	commit(t, s, preview)
	waitStatus(t, r, statusRunning)
	return s, r, newer
}

func TestAListedRepositoryIsOfferedOnlyWhatTheCatalogLists(t *testing.T) {
	s, r, _ := installedListed(t)
	if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates() = %v", err)
	}
	if got := r.updatesSeen(); len(got) != 0 {
		t.Fatalf("updates = %+v while the catalog lists the installed version, want none", got)
	}
	_, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err == nil || !strings.Contains(err.Error(), "the lich catalog lists 1.0.0, which is not a newer version") {
		t.Errorf("PrepareUpdate() = %v, want it refused by the catalog's version", err)
	}
}

func TestAListedRepositoryUpdatesToTheCatalogsTag(t *testing.T) {
	s, r, newer := installedListed(t)
	listAs(t, s, listedAt(newerTag, newerVersion, map[string]string{hostPlatform(): sha(newer)}))
	github := s.releaseAPI
	s.releaseAPI = "http://127.0.0.1:1/repos/"
	if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates() = %v, want no GitHub read for a listed repository", err)
	}
	if got := only(t, s).Latest; got != newerTag {
		t.Fatalf("latest = %q, want the catalog's %s", got, newerTag)
	}
	waitUntil(t, "the update offer", func() bool { return len(r.updatesSeen()) == 1 })
	s.releaseAPI = github
	preview, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err != nil {
		t.Fatalf("PrepareUpdate() = %v", err)
	}
	if got := commit(t, s, preview); got.Version != newerVersion || got.Release != newerTag {
		t.Errorf("updated = %+v, want %s from %s", got, newerVersion, newerTag)
	}
}

func TestAListedUpdateRefusesAnArchiveReplacedSince(t *testing.T) {
	s, _, _ := installedListed(t)
	listAs(t, s, listedAt(newerTag, newerVersion, map[string]string{hostPlatform(): sha([]byte("what the catalog saw"))}))
	_, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err == nil || !strings.Contains(err.Error(), "differs from the archive the lich catalog recorded") {
		t.Fatalf("PrepareUpdate() = %v, want the archive refused", err)
	}
	if got := only(t, s); got.Version != "1.0.0" || got.Status != statusRunning {
		t.Errorf("installed = %+v, want 1.0.0 still running", got)
	}
}
