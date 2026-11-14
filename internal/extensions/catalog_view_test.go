package extensions

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

func TestCatalogSaysWhyWhileItHoldsNoneAndReadsAgainWhenAsked(t *testing.T) {
	s, f := catalogService(t)
	f.fail(http.StatusBadGateway)
	if _, err := s.Catalog(context.Background(), api.None{}); err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("Catalog() = %v, want the read's error", err)
	}
	f.publish(t, catalogOf(extension.CatalogEntry{ID: "one", Repo: testRepo}))
	got, err := s.Catalog(context.Background(), api.None{})
	if err != nil {
		t.Fatalf("Catalog() = %v", err)
	}
	if len(got.Catalog.Extensions) != 1 || got.Catalog.Extensions[0].ID != "one" || got.Platform != hostPlatform() {
		t.Errorf("Catalog() = %+v, want one on %s", got, hostPlatform())
	}
	f.fail(http.StatusBadGateway)
	if _, err := s.Catalog(context.Background(), api.None{}); err != nil {
		t.Errorf("Catalog() = %v with a copy held, want the copy", err)
	}
}

func TestOnlyANewCopyOfTheCatalogIsAnnounced(t *testing.T) {
	s, r := boot(t, t.TempDir())
	f := &fakeCatalog{}
	s.catalogURL = f.serve(t)
	f.publish(t, catalogOf(extension.CatalogEntry{ID: "one", Repo: testRepo}))
	for range 2 {
		if err := s.readCatalog(); err != nil {
			t.Fatal(err)
		}
	}
	f.publish(t, catalogOf(extension.CatalogEntry{ID: "two", Repo: testRepo}))
	if err := s.readCatalog(); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.catalogs) != 2 || r.catalogs[1].Catalog.Extensions[0].ID != "two" || r.catalogs[1].Platform != hostPlatform() {
		t.Errorf("announced %+v, want the first copy and the changed one", r.catalogs)
	}
}

// imageHost serves images by path and counts every request.
func imageHost(t *testing.T, images map[string][]byte) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		data, ok := images[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

func TestCatalogImageReadsOnlyWhatTheCatalogNamesWithinItsLimit(t *testing.T) {
	shot := bytes.Repeat([]byte{1}, extension.MaxCoverBytes+1)
	host, hits := imageHost(t, map[string][]byte{
		"/cover.png": []byte("cover"), "/shot.png": shot, "/big.png": shot, "/gone.png": nil,
	})
	s, f := catalogService(t)
	f.publish(t, catalogOf(extension.CatalogEntry{
		ID: "one", Repo: testRepo, Cover: host + "/cover.png", Icon: host + "/big.png",
		Screenshots: []extension.CatalogImage{{URL: host + "/shot.png"}, {URL: host + "/missing.png"}},
	}))
	if _, err := s.Catalog(context.Background(), api.None{}); err != nil {
		t.Fatal(err)
	}
	read := func(path string) ([]byte, error) {
		return s.CatalogImage(context.Background(), api.ExtensionCatalogImageOptions{URL: host + path})
	}
	if got, err := read("/cover.png"); err != nil || string(got) != "cover" {
		t.Errorf("cover = %q, %v", got, err)
	}
	if got, err := read("/shot.png"); err != nil || len(got) != len(shot) {
		t.Errorf("screenshot = %d bytes, %v; want it whole, under the screenshot limit", len(got), err)
	}
	if _, err := read("/big.png"); err == nil || !strings.Contains(err.Error(), "larger than the 1 MiB") {
		t.Errorf("icon over the cover's limit = %v, want it refused", err)
	}
	if _, err := read("/missing.png"); err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Errorf("missing = %v, want the status", err)
	}
	before := hits.Load()
	if _, err := read("/gone.png"); err == nil || !strings.Contains(err.Error(), "not an image the lich catalog names") {
		t.Errorf("unnamed = %v, want it refused", err)
	}
	if hits.Load() != before {
		t.Error("an address the catalog does not name was fetched")
	}
}

func TestAPreviewNamesItsPublisherAndWhetherItIsListed(t *testing.T) {
	older, _, s, _, _ := twoReleases(t)
	entry := listedAt(testTag, "1.0.0", map[string]string{hostPlatform(): sha(older)})
	entry.Publisher = "SomeOne"
	listAs(t, s, entry)
	preview, err := s.PrepareFromGit(context.Background(), api.ExtensionGitInstallOptions{URL: testRepo})
	if err != nil {
		t.Fatalf("PrepareFromGit() = %v", err)
	}
	if preview.Publisher != "SomeOne" || !preview.Listed {
		t.Errorf("preview = %+v, want the catalog's publisher, listed", preview)
	}
}

func TestARevokedExtensionNamesTheReasonApartFromItsError(t *testing.T) {
	s, _, _ := revokedInstall(t)
	if got := only(t, s); got.Revoked != revokedReasonText {
		t.Errorf("Revoked = %q, want %q", got.Revoked, revokedReasonText)
	}
}
