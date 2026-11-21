package themes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lichdotdev/lich/api"
)

// catalogService is a theme service reading cat from a local server that
// answers If-None-Match with 304 while the catalog has not changed. hits
// counts the requests that reached it.
func catalogService(t *testing.T, cat *api.ThemeCatalog, hits *atomic.Int32) (*Service, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if cat == nil {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_ = json.NewEncoder(w).Encode(cat)
	}))
	t.Cleanup(srv.Close)
	s := NewInDir(t.TempDir())
	s.catalogURL, s.catalogHTTP = srv.URL, srv.Client()
	return s, srv
}

func headCommit(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func listing(repo, commit, version string) api.ThemeCatalogPack {
	return api.ThemeCatalogPack{ID: "sample", Name: "Sample pack", Repo: repo, Commit: commit, Version: version}
}

func TestCatalogReadsOnceAndKeepsItsCopyWhileTheServerSaysNotModified(t *testing.T) {
	var hits atomic.Int32
	var sent []string
	cat := &api.ThemeCatalog{FormatVersion: 1, Packs: []api.ThemeCatalogPack{{ID: "rose", Name: "Rosé Pine"}}}
	s, _ := catalogService(t, cat, &hits)
	s.emit = func(name string, _ any) { sent = append(sent, name) }
	got, err := s.Catalog(t.Context(), api.None{})
	if err != nil || len(got.Packs) != 1 || got.Packs[0].ID != "rose" {
		t.Fatalf("catalog %+v, %v", got, err)
	}
	if _, err := s.Catalog(t.Context(), api.None{}); err != nil || hits.Load() != 1 {
		t.Errorf("a second call read %d times, err %v: the copy in memory answers", hits.Load(), err)
	}
	s.refreshCatalog()
	if hits.Load() != 2 || len(sent) != 1 || sent[0] != api.ThemeCatalogEvent.Name {
		t.Errorf("hits %d, events %v: a 304 keeps the copy and tells the window nothing", hits.Load(), sent)
	}
}

func TestCatalogFailsOnlyWhenItHasNoCopy(t *testing.T) {
	var hits atomic.Int32
	s, _ := catalogService(t, nil, &hits)
	if _, err := s.Catalog(t.Context(), api.None{}); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("err %v, want the server's status", err)
	}
	newer := &api.ThemeCatalog{FormatVersion: catalogFormat + 1}
	s2, _ := catalogService(t, newer, &hits)
	if _, err := s2.Catalog(t.Context(), api.None{}); err == nil || !strings.Contains(err.Error(), "format") {
		t.Errorf("err %v: a format newer than this lich reads is refused", err)
	}
}

func TestInstallFromCatalogInstallsTheListedCommitNotTheTip(t *testing.T) {
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
	})
	listed := headCommit(t, repo)
	writeFile(t, repo, manifestName, manifestJSON(t, "Sample pack", "1.1.0"))
	commitAll(t, repo, "bump")
	var hits atomic.Int32
	s, _ := catalogService(t, &api.ThemeCatalog{FormatVersion: 1, Packs: []api.ThemeCatalogPack{listing(repo, listed, "1.0.0")}}, &hits)
	result, err := s.InstallFromCatalog(t.Context(), api.ThemeCatalogInstallOptions{ID: "sample"})
	if err != nil {
		t.Fatalf("InstallFromCatalog: %v", err)
	}
	if result.Version != "1.0.0" || len(result.Themes) != 1 || result.Themes[0].Source.URL != repo {
		t.Errorf("result %+v: want 1.0.0, the listed commit, sourced from the repository", result)
	}
}

func TestInstallFromCatalogRefusesWhatTheCatalogDoesNotVouchFor(t *testing.T) {
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
	})
	head := headCommit(t, repo)
	var hits atomic.Int32
	cat := &api.ThemeCatalog{FormatVersion: 1, Packs: []api.ThemeCatalogPack{
		listing(repo, head, "2.0.0"),
		{ID: "gone", Name: "Gone", Repo: "https://github.com/o/gone-lich", Commit: head, Version: "1.0.0"},
		{ID: "short", Name: "Short", Repo: repo, Commit: head[:12], Version: "1.0.0"},
	}, Revoked: []api.ThemeRevocation{{Repo: "https://github.com/O/gone-lich.git", Reason: "stolen"}}}
	s, _ := catalogService(t, cat, &hits)
	for id, want := range map[string]string{
		"sample": "not the 2.0.0", "gone": "took Gone down", "short": "not a full commit", "missing": `no pack "missing"`,
	} {
		if _, err := s.InstallFromCatalog(t.Context(), api.ThemeCatalogInstallOptions{ID: id}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err %v, want %q", id, err, want)
		}
	}
}

func TestAPackFromTheCatalogUpdatesOnlyToWhatTheCatalogOffers(t *testing.T) {
	repo := themeRepo(t, map[string]string{
		manifestName:       manifestJSON(t, "Sample pack", "1.0.0"),
		"tokyo-night.json": rawTheme(t, customTheme("tokyo-night")),
	})
	cat := &api.ThemeCatalog{FormatVersion: 1, Packs: []api.ThemeCatalogPack{listing(repo, headCommit(t, repo), "1.0.0")}}
	var hits atomic.Int32
	s, _ := catalogService(t, cat, &hits)
	if _, err := s.InstallFromCatalog(t.Context(), api.ThemeCatalogInstallOptions{ID: "sample"}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, manifestName, manifestJSON(t, "Sample pack", "1.1.0"))
	commitAll(t, repo, "bump, not listed yet")
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatal(err)
	}
	if themes, _ := s.List(t.Context(), api.None{}); latestIn(themes, "tokyo-night") != nil {
		t.Error("an unlisted tip is offered as an update")
	}
	if r, err := s.UpdateFromGit(t.Context(), api.UpdateThemeOptions{ID: "tokyo-night"}); err != nil || !r.UpToDate {
		t.Errorf("update %+v, %v: the catalog still lists 1.0.0", r, err)
	}
	cat.Packs[0].Version, cat.Packs[0].Commit = "1.1.0", headCommit(t, repo)
	s.catalog.Store(cat)
	if _, err := s.CheckUpdates(t.Context(), api.None{}); err != nil {
		t.Fatal(err)
	}
	if themes, _ := s.List(t.Context(), api.None{}); latestIn(themes, "tokyo-night") == nil {
		t.Error("the listed 1.1.0 is not offered")
	}
	if r, err := s.UpdateFromGit(t.Context(), api.UpdateThemeOptions{ID: "tokyo-night"}); err != nil || r.Version != "1.1.0" {
		t.Errorf("update %+v, %v: want the listed 1.1.0", r, err)
	}
}

func latestIn(themes []Theme, id string) *api.ThemeLatest {
	for _, t := range themes {
		if t.ID == id {
			return t.Latest
		}
	}
	return nil
}

func TestARepositoryIsTheSameWhateverItsCaseOrEnding(t *testing.T) {
	for _, url := range []string{"https://github.com/O/Rose-lich", "https://github.com/o/rose-lich.git", " https://github.com/o/rose-lich/ "} {
		if sameRepo(url) != sameRepo("https://github.com/o/rose-lich") {
			t.Errorf("%q is not the same repository", url)
		}
	}
}
