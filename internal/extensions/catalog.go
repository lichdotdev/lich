package extensions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lichdotdev/lich/extension"
)

const (
	catalogURL = "https://raw.githubusercontent.com/lichdotdev/extensions/main/catalog.json"
	// maxCatalogBytes bounds catalog.json, a few KiB per listed extension.
	maxCatalogBytes = 4 << 20
)

// refreshCatalog reads the catalog, logging a failure: lich then goes on
// with the last good copy, or with none.
func (s *Service) refreshCatalog() {
	if err := s.readCatalog(); err != nil {
		slog.Warn("extensions: read the catalog", "err", err)
	}
}

// catalogNow is the catalog in memory, read first when none has been.
func (s *Service) catalogNow() *extension.Catalog {
	if cat := s.catalog.Load(); cat != nil {
		return cat
	}
	s.refreshCatalog()
	return s.catalog.Load()
}

// readCatalog replaces the copy in memory with catalog.json, unless the
// server says it has not changed. A read that fails, or a catalog this lich
// cannot read, keeps the copy it has.
func (s *Service) readCatalog() error {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	etag := ""
	if s.catalog.Load() != nil {
		etag = s.catalogTag
	}
	cat, etag, err := s.fetchCatalog(etag)
	if err != nil || cat == nil {
		return err
	}
	s.catalog.Store(cat)
	s.catalogTag = etag
	return nil
}

// fetchCatalog is nil with no error when the copy etag names is current.
func (s *Service) fetchCatalog(etag string) (*extension.Catalog, string, error) {
	req, err := http.NewRequest(http.MethodGet, s.catalogURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "lich")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := s.metadata.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotModified {
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var body bytes.Buffer
	n, err := io.Copy(&body, io.LimitReader(resp.Body, maxCatalogBytes+1))
	if err != nil {
		return nil, "", err
	}
	if n > maxCatalogBytes {
		return nil, "", fmt.Errorf("larger than the %d MiB lich accepts", maxCatalogBytes>>20)
	}
	var cat extension.Catalog
	if err := json.Unmarshal(body.Bytes(), &cat); err != nil {
		return nil, "", fmt.Errorf("not a catalog: %w", err)
	}
	if cat.FormatVersion < 1 || cat.FormatVersion > extension.CatalogFormat {
		return nil, "", fmt.Errorf("format %d is not one this lich reads (up to %d)", cat.FormatVersion, extension.CatalogFormat)
	}
	return &cat, resp.Header.Get("ETag"), nil
}

// listing is the entry the catalog lists repoURL under, nil when it lists
// none. GitHub's owner and repository names ignore case, and so does this.
func listing(cat *extension.Catalog, repoURL string) *extension.CatalogEntry {
	if cat == nil {
		return nil
	}
	for i := range cat.Extensions {
		if strings.EqualFold(cat.Extensions[i].Repo, repoURL) {
			return &cat.Extensions[i]
		}
	}
	return nil
}
