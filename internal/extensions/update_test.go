package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

const (
	newerTag     = "v1.1.0"
	newerVersion = "1.1.0"
)

// installedFromGit is testID installed and running from testTag.
func installedFromGit(t *testing.T) (*Service, *recorder, string) {
	t.Helper()
	srv := releaseServer(t, signed(map[string][]byte{hostAsset(): tarball(t, extensionEntries(t, testID, selfBytes(t)))}))
	s, r, dir := installer(t, srv)
	preview, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err != nil {
		t.Fatalf("PrepareFromGit() = %v", err)
	}
	commit(t, s, preview)
	waitStatus(t, r, statusRunning)
	return s, r, dir
}

// publishNewer points s at a server whose latest release is newerTag, holding
// archive for this machine.
func publishNewer(t *testing.T, s *Service, archive []byte) {
	t.Helper()
	s.releaseAPI = releaseServerAt(t, newerTag, signed(map[string][]byte{hostAsset(): archive})).URL + "/repos/"
}

func newerArchive(t *testing.T, id string) []byte {
	t.Helper()
	m := validManifest(id, testExecutable())
	m.Version = newerVersion
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return tarball(t, []tarEntry{
		{name: extension.ManifestFile, body: raw},
		{name: testExecutable(), body: selfBytes(t)},
		{name: modeFile, body: []byte(modeEcho)},
	})
}

func (r *recorder) updatesSeen() []api.ExtensionUpdate {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.updates)
}

func only(t *testing.T, s *Service) api.Extension {
	t.Helper()
	list, _ := s.List(context.Background(), api.None{})
	if len(list) != 1 {
		t.Fatalf("List() = %+v, want one extension", list)
	}
	return list[0]
}

func TestCheckUpdatesTellsTheWindowOnceOfANewerRelease(t *testing.T) {
	s, r, _ := installedFromGit(t)
	if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates() = %v", err)
	}
	if got := r.updatesSeen(); len(got) != 0 {
		t.Fatalf("updates = %+v with the latest release installed, want none", got)
	}
	publishNewer(t, s, newerArchive(t, testID))
	for range 2 {
		if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
			t.Fatalf("CheckUpdates() = %v", err)
		}
	}
	want := []api.ExtensionUpdate{{ID: testID, Latest: newerTag}}
	if got := r.updatesSeen(); !reflect.DeepEqual(got, want) {
		t.Fatalf("updates = %+v, want %+v", got, want)
	}
	if got := only(t, s); got.Latest != newerTag || got.Release != testTag {
		t.Fatalf("List() = %+v, want release %s with %s waiting", got, testTag, newerTag)
	}
}

func TestCheckUpdatesOffersOnlyALaterVersion(t *testing.T) {
	for _, tag := range []string{"v0.9.0", "nightly", testTag} {
		t.Run(tag, func(t *testing.T) {
			s, r, _ := installedFromGit(t)
			s.releaseAPI = releaseServerAt(t, tag, signed(map[string][]byte{hostAsset(): newerArchive(t, testID)})).URL + "/repos/"
			if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
				t.Fatalf("CheckUpdates() = %v", err)
			}
			if got := r.updatesSeen(); len(got) != 0 {
				t.Fatalf("updates = %+v for latest release %s over %s, want none", got, tag, testTag)
			}
		})
	}
}

func TestCheckUpdatesSkipsAnExtensionWithNoRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "extensions")
	install(t, dir, "byhand", modeEcho)
	s, r := boot(t, dir)
	s.releaseAPI = "http://127.0.0.1:1/unreachable/"
	if _, err := s.CheckUpdates(context.Background(), api.None{}); err != nil {
		t.Fatalf("CheckUpdates() = %v, want no request made", err)
	}
	if got := r.updatesSeen(); len(got) != 0 {
		t.Fatalf("updates = %+v, want none", got)
	}
}

func TestCheckUpdatesReportsARepositoryItCannotRead(t *testing.T) {
	s, _, _ := installedFromGit(t)
	s.releaseAPI = releaseServer(t, nil).URL + "/repos/"
	if _, err := s.CheckUpdates(context.Background(), api.None{}); err == nil {
		t.Fatal("CheckUpdates() = nil, want the lookup's error")
	}
}

func TestUpdateReplacesTheInstalledVersionAndKeepsItsData(t *testing.T) {
	s, r, dir := installedFromGit(t)
	kept := filepath.Join(s.dataDir, testID, "notes")
	writeFile(t, kept, "mine")
	publishNewer(t, s, newerArchive(t, testID))

	preview, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err != nil {
		t.Fatalf("PrepareUpdate() = %v", err)
	}
	if preview.Version != newerVersion || preview.Release != newerTag || preview.Source != testRepo {
		t.Fatalf("PrepareUpdate() = %+v, want %s at %s from %s", preview, newerVersion, newerTag, testRepo)
	}
	if got := only(t, s); got.Version != "1.0.0" || got.Status != statusRunning || got.Latest != newerTag {
		t.Fatalf("List() = %+v after prepare, want the installed version still running with %s waiting", got, newerTag)
	}

	got := commit(t, s, preview)
	if got.Version != newerVersion || got.Release != newerTag || got.Latest != "" {
		t.Fatalf("CommitInstall() = %+v, want %s at %s and nothing newer", got, newerVersion, newerTag)
	}
	waitUntil(t, "the update running", func() bool {
		last := r.lastStatus()
		return last.Status == statusRunning && last.Version == newerVersion
	})
	wantUpdates := []api.ExtensionUpdate{{ID: testID, Latest: newerTag}, {ID: testID}}
	if got := r.updatesSeen(); !reflect.DeepEqual(got, wantUpdates) {
		t.Fatalf("updates = %+v, want the offer and then its end: %+v", got, wantUpdates)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != testID {
		t.Fatalf("extensions dir = %v, %v; want only %s", entries, err, testID)
	}
	if raw, err := os.ReadFile(kept); err != nil || string(raw) != "mine" {
		t.Fatalf("data = %q, %v; want it kept", raw, err)
	}
	if again := load(filepath.Join(dir, testID), testID, "1.2.3"); again.source != (source{URL: testRepo, Release: newerTag}) {
		t.Fatalf("a later discovery reads source %+v", again.source)
	}
}

func TestPrepareUpdateRefusesWhatHasNothingNewer(t *testing.T) {
	s, _, _ := installedFromGit(t)
	_, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err == nil || !strings.Contains(err.Error(), "not a newer version") {
		t.Fatalf("PrepareUpdate() = %v, want it to say the latest is installed", err)
	}
	if got := stagedDirs(t, s.dir); len(got) != 0 {
		t.Fatalf("staged %v, want nothing", got)
	}
}

func TestPrepareUpdateRefusesAnExtensionNotFromGitHub(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "extensions")
	install(t, dir, "byhand", modeEcho)
	s, _ := boot(t, dir)
	for _, id := range []string{"byhand", "missing"} {
		if _, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: id}); err == nil {
			t.Fatalf("PrepareUpdate(%q) = nil, want an error", id)
		}
	}
}

func TestAnUpdateThatFailsLeavesTheInstalledVersionRunning(t *testing.T) {
	cases := map[string]func(t *testing.T) []byte{
		"another extension's archive": func(t *testing.T) []byte { return newerArchive(t, "other") },
		"an archive that is not one":  func(*testing.T) []byte { return []byte("not a tarball") },
	}
	for name, archive := range cases {
		t.Run(name, func(t *testing.T) {
			s, _, _ := installedFromGit(t)
			publishNewer(t, s, archive(t))
			if _, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID}); err == nil {
				t.Fatal("PrepareUpdate() = nil, want an error")
			}
			if got := only(t, s); got.Version != "1.0.0" || got.Status != statusRunning {
				t.Fatalf("List() = %+v, want the installed version still running", got)
			}
			if got := stagedDirs(t, s.dir); len(got) != 0 {
				t.Fatalf("staged %v, want nothing left", got)
			}
		})
	}
}

func TestAnUpdatePreparedForAnExtensionRemovedSinceIsRefused(t *testing.T) {
	s, _, dir := installedFromGit(t)
	publishNewer(t, s, newerArchive(t, testID))
	preview, err := s.PrepareUpdate(context.Background(), api.ExtensionIDOptions{ID: testID})
	if err != nil {
		t.Fatalf("PrepareUpdate() = %v", err)
	}
	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, err := s.CommitInstall(context.Background(), api.ExtensionInstallTokenOptions{Token: preview.Token}); err == nil {
		t.Fatal("CommitInstall() = nil, want the update refused")
	}
	assertNothingInstalled(t, s, dir)
}
