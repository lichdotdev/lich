package extensions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/extension"
)

// fakeCatalog is raw.githubusercontent.com serving catalog.json, with an ETag
// for each body.
type fakeCatalog struct {
	mu sync.Mutex
	// status, when set, is the answer to every read.
	status      int
	body        []byte
	full        int
	notModified int
}

func (f *fakeCatalog) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.status != 0 {
			w.WriteHeader(f.status)
			return
		}
		sum := sha256.Sum256(f.body)
		etag := `"` + hex.EncodeToString(sum[:8]) + `"`
		if r.Header.Get("If-None-Match") == etag {
			f.notModified++
			w.WriteHeader(http.StatusNotModified)
			return
		}
		f.full++
		w.Header().Set("ETag", etag)
		_, _ = w.Write(f.body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/catalog.json"
}

func (f *fakeCatalog) publish(t *testing.T, cat extension.Catalog) {
	t.Helper()
	raw, err := json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	f.serveRaw(raw)
}

func (f *fakeCatalog) serveRaw(raw []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status, f.body = 0, raw
}

func (f *fakeCatalog) fail(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

func (f *fakeCatalog) reads() (full, notModified int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.full, f.notModified
}

func catalogOf(entries ...extension.CatalogEntry) extension.Catalog {
	return extension.Catalog{FormatVersion: extension.CatalogFormat, Extensions: entries}
}

// catalogService is a Service reading f's catalog.
func catalogService(t *testing.T) (*Service, *fakeCatalog) {
	t.Helper()
	s, _ := boot(t, t.TempDir())
	f := &fakeCatalog{}
	s.catalogURL = f.serve(t)
	return s, f
}

func TestReadCatalogAsksOnlyWhetherTheCopyHeldChanged(t *testing.T) {
	s, f := catalogService(t)
	f.publish(t, catalogOf(extension.CatalogEntry{ID: "one", Repo: testRepo}))
	for range 2 {
		if err := s.readCatalog(); err != nil {
			t.Fatalf("readCatalog() = %v", err)
		}
	}
	if full, notModified := f.reads(); full != 1 || notModified != 1 {
		t.Fatalf("reads = %d full, %d not modified; want one of each", full, notModified)
	}
	f.publish(t, catalogOf(extension.CatalogEntry{ID: "two", Repo: testRepo}))
	if err := s.readCatalog(); err != nil {
		t.Fatalf("readCatalog() = %v", err)
	}
	if got := s.catalog.Load().Extensions[0].ID; got != "two" {
		t.Errorf("catalog lists %q after it changed, want two", got)
	}
}

func TestReadCatalogKeepsTheLastGoodCopy(t *testing.T) {
	newer := catalogOf()
	newer.FormatVersion = extension.CatalogFormat + 1
	newerRaw, _ := json.Marshal(newer)
	cases := []struct {
		name  string
		serve func(*fakeCatalog)
		want  string
	}{
		{"a failed read", func(f *fakeCatalog) { f.fail(http.StatusInternalServerError) }, "status 500"},
		{"not JSON", func(f *fakeCatalog) { f.serveRaw([]byte("<html>")) }, "not a catalog"},
		{"a newer format", func(f *fakeCatalog) { f.serveRaw(newerRaw) }, "format 2"},
		{"no format", func(f *fakeCatalog) { f.serveRaw([]byte(`{"extensions":[]}`)) }, "format 0"},
		{"too large", func(f *fakeCatalog) { f.serveRaw([]byte(strings.Repeat(" ", maxCatalogBytes+1))) }, "larger than"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, f := catalogService(t)
			f.publish(t, catalogOf(extension.CatalogEntry{ID: "kept", Repo: testRepo}))
			if err := s.readCatalog(); err != nil {
				t.Fatalf("readCatalog() = %v", err)
			}
			c.serve(f)
			if err := s.readCatalog(); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("readCatalog() = %v, want an error naming %q", err, c.want)
			}
			if cat := s.catalog.Load(); cat == nil || cat.Extensions[0].ID != "kept" {
				t.Errorf("catalog = %+v, want the last good copy", cat)
			}
		})
	}
}

func TestACatalogOfANewerFormatIsNeverHeld(t *testing.T) {
	s, f := catalogService(t)
	newer := catalogOf(extension.CatalogEntry{ID: "one", Repo: testRepo})
	newer.FormatVersion = extension.CatalogFormat + 1
	f.publish(t, newer)
	if cat := s.catalogNow(); cat != nil {
		t.Errorf("catalogNow() = %+v, want none", cat)
	}
}

func TestCatalogNowReadsOnlyWhenNoneIsHeld(t *testing.T) {
	s, f := catalogService(t)
	f.publish(t, catalogOf())
	s.catalogNow()
	s.catalogNow()
	if full, notModified := f.reads(); full != 1 || notModified != 0 {
		t.Errorf("reads = %d full, %d not modified; want the one", full, notModified)
	}
}

func TestWatchUpdatesReadsTheCatalogAtStart(t *testing.T) {
	s, f := catalogService(t)
	f.publish(t, catalogOf())
	s.WatchUpdates()
	waitUntil(t, "the catalog read", func() bool { return s.catalog.Load() != nil })
}
