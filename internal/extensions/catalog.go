package extensions

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/events"
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
	s.enforceRevocations()
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
// server says it has not changed, and tells the window of a new copy. A read
// that fails, or a catalog this lich cannot read, keeps the copy it has.
func (s *Service) readCatalog() error {
	cat, err := s.storeCatalog()
	if cat != nil {
		events.Send(s.emit, api.ExtensionCatalogEvent, s.catalogView(cat))
	}
	return err
}

// storeCatalog is readCatalog's read, the copy it stored or nil.
func (s *Service) storeCatalog() (*extension.Catalog, error) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	etag := ""
	if s.catalog.Load() != nil {
		etag = s.catalogTag
	}
	cat, etag, err := s.fetchCatalog(etag)
	if err != nil || cat == nil {
		return nil, err
	}
	s.catalog.Store(cat)
	s.catalogTag = etag
	return cat, nil
}

func (s *Service) catalogView(cat *extension.Catalog) api.ExtensionCatalog {
	return api.ExtensionCatalog{Catalog: *cat, Platform: s.goos + "_" + s.goarch}
}

// Catalog is the catalog in memory, read now when there is none; the read's
// error is the answer only when that leaves none.
func (s *Service) Catalog(context.Context, api.None) (api.ExtensionCatalog, error) {
	cat := s.catalog.Load()
	if cat == nil {
		err := s.readCatalog()
		s.enforceRevocations()
		if cat = s.catalog.Load(); cat == nil {
			return api.ExtensionCatalog{}, cmp.Or(err, errors.New("the lich catalog could not be read"))
		}
	}
	return s.catalogView(cat), nil
}

// CatalogImage reads an image the catalog in memory names, held to the
// listing's limit for it: the window draws what the catalog shows through
// here, and no other address is fetched on its behalf.
func (s *Service) CatalogImage(_ context.Context, opts api.ExtensionCatalogImageOptions) ([]byte, error) {
	limit := imageLimit(s.catalog.Load(), opts.URL)
	if limit == 0 {
		return nil, fmt.Errorf("%q is not an image the lich catalog names", opts.URL)
	}
	req, err := http.NewRequest(http.MethodGet, opts.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lich")
	resp, err := s.download.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", opts.URL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than the %d MiB the catalog allows", opts.URL, limit>>20)
	}
	return data, nil
}

// imageLimit is the most bytes the catalog lets the image at url take, 0 for
// a URL it does not name. An icon has no limit of its own and is held to the
// cover's, the smaller one.
func imageLimit(cat *extension.Catalog, url string) int64 {
	if cat == nil || url == "" {
		return 0
	}
	for _, e := range cat.Extensions {
		if e.Cover == url || e.Icon == url {
			return extension.MaxCoverBytes
		}
		for _, shot := range e.Screenshots {
			if shot.URL == url {
				return extension.MaxScreenshotBytes
			}
		}
	}
	return 0
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

// revokedReason is why the catalog took repoURL down, "" when it did not.
func revokedReason(cat *extension.Catalog, repoURL string) string {
	if cat == nil {
		return ""
	}
	for _, r := range cat.Revoked {
		if strings.EqualFold(r.Repo, repoURL) {
			return cmp.Or(r.Reason, "no reason given")
		}
	}
	return ""
}

func revokedError(reason string) error {
	return fmt.Errorf("removed from the lich catalog: %s", reason)
}

// enforceRevocations stops every installed extension whose repository the
// catalog in memory took down, for good.
func (s *Service) enforceRevocations() {
	cat := s.catalog.Load()
	if cat == nil || len(cat.Revoked) == 0 {
		return
	}
	s.mu.Lock()
	all := slices.Clone(s.all)
	s.mu.Unlock()
	for _, x := range all {
		if reason := revokedReason(cat, x.source.URL); reason != "" {
			s.revoke(x, reason)
		}
	}
}

// revoke stops x and lists it invalid with reason. restartMu keeps a
// Restart from starting it again on the way.
func (s *Service) revoke(x *installed, reason string) {
	x.restartMu.Lock()
	defer x.restartMu.Unlock()
	s.mu.Lock()
	if s.find(x.id) != x || x.source.Revoked != "" {
		s.mu.Unlock()
		return
	}
	markRevoked(x, reason)
	stop, done := x.stop, x.done
	s.mu.Unlock()
	if stop != nil {
		stop()
		<-done
	}
	s.setStatus(x, statusInvalid, x.invalid.Error())
}

// revokeLoaded marks x, read from disk and not yet started, when the catalog
// took its repository down after its install was prepared. Called with mu
// held.
func (s *Service) revokeLoaded(x *installed) {
	if reason := revokedReason(s.catalog.Load(), x.source.URL); reason != "" && x.source.Revoked == "" {
		markRevoked(x, reason)
		x.status, x.errText = statusInvalid, x.invalid.Error()
	}
}

// markRevoked records the revocation in x's sourceFile, which is what keeps
// it stopped across restarts. Called with mu held.
func markRevoked(x *installed, reason string) {
	x.source.Revoked = reason
	x.invalid = revokedError(reason)
	slog.Warn("extensions: removed from the lich catalog", "extension", x.id, "reason", reason)
	if err := writeSource(x.root, x.source); err != nil {
		slog.Warn("extensions: record the revocation", "extension", x.id, "err", err)
	}
}
