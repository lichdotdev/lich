package themes

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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/gitutil"
	"github.com/lichdotdev/lich/internal/winexec"
)

const (
	catalogURL = "https://raw.githubusercontent.com/lichdotdev/themes/main/catalog.json"
	// catalogFormat is the newest catalog.json format this lich reads.
	catalogFormat = 1
	// maxCatalogBytes bounds catalog.json: every listed theme in full is
	// about 5 KiB.
	maxCatalogBytes = 8 << 20
	catalogTimeout  = 30 * time.Second
)

// commitPattern is a full git object name: a catalog pins a pack by it, and
// git fetches nothing else by it.
var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// refreshCatalog reads the catalog, logging a failure: lich then goes on with
// the last good copy, or with none.
func (s *Service) refreshCatalog() {
	if err := s.readCatalog(); err != nil {
		slog.Warn("themes: read the catalog", "err", err)
	}
}

// readCatalog replaces the copy in memory with catalog.json, unless the
// server says it has not changed, and tells the window of a new copy. A read
// that fails, or a catalog this lich cannot read, keeps the copy it has.
func (s *Service) readCatalog() error {
	s.catalogMu.Lock()
	etag := ""
	if s.catalog.Load() != nil {
		etag = s.catalogTag
	}
	cat, etag, err := s.fetchCatalog(etag)
	if cat != nil {
		s.catalog.Store(cat)
		s.catalogTag = etag
	}
	s.catalogMu.Unlock()
	if cat != nil {
		events.Send(s.emit, api.ThemeCatalogEvent, *cat)
	}
	return err
}

// Catalog is the catalog in memory, read now when there is none; the read's
// error is the answer only when that leaves none.
func (s *Service) Catalog(context.Context, api.None) (api.ThemeCatalog, error) {
	cat := s.catalog.Load()
	if cat == nil {
		err := s.readCatalog()
		if cat = s.catalog.Load(); cat == nil {
			return api.ThemeCatalog{}, cmp.Or(err, errors.New("the theme catalog could not be read"))
		}
	}
	return *cat, nil
}

// fetchCatalog is nil with no error when the copy etag names is current.
func (s *Service) fetchCatalog(etag string) (*api.ThemeCatalog, string, error) {
	req, err := http.NewRequest(http.MethodGet, s.catalogURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "lich")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := s.catalogHTTP.Do(req)
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
	var cat api.ThemeCatalog
	if err := json.Unmarshal(body.Bytes(), &cat); err != nil {
		return nil, "", fmt.Errorf("not a theme catalog: %w", err)
	}
	if cat.FormatVersion < 1 || cat.FormatVersion > catalogFormat {
		return nil, "", fmt.Errorf("format %d is not one this lich reads (up to %d)", cat.FormatVersion, catalogFormat)
	}
	return &cat, resp.Header.Get("ETag"), nil
}

// listedPack is the pack the catalog in memory lists repoURL under, nil when
// it lists none or took it down. GitHub ignores case in owner and repository
// names, and a remote may end in .git or a slash; so does this.
func (s *Service) listedPack(repoURL string) *api.ThemeCatalogPack {
	cat := s.catalog.Load()
	if cat == nil {
		return nil
	}
	want := sameRepo(repoURL)
	for _, r := range cat.Revoked {
		if sameRepo(r.Repo) == want {
			return nil
		}
	}
	for i := range cat.Packs {
		if sameRepo(cat.Packs[i].Repo) == want {
			return &cat.Packs[i]
		}
	}
	return nil
}

func sameRepo(url string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git"))
}

// InstallFromCatalog installs the pack the catalog lists under opts.ID, from
// its repository at the listed commit, and refuses it when the manifest there
// is not the listed version.
func (s *Service) InstallFromCatalog(ctx context.Context, opts api.ThemeCatalogInstallOptions) (GitInstallResult, error) {
	if s.initErr != nil {
		return GitInstallResult{}, s.initErr
	}
	cat, err := s.Catalog(ctx, api.None{})
	if err != nil {
		return GitInstallResult{}, err
	}
	for _, p := range cat.Packs {
		if p.ID != opts.ID {
			continue
		}
		if s.listedPack(p.Repo) == nil {
			return GitInstallResult{}, fmt.Errorf("the catalog took %s down", p.Name)
		}
		installed, err := fetchListed(p, s.lichVersion)
		if err != nil {
			return GitInstallResult{}, err
		}
		return s.installPack(installed, opts.Overwrite)
	}
	return GitInstallResult{}, fmt.Errorf("the theme catalog lists no pack %q", opts.ID)
}

// fetchListed reads p's repository at its listed commit, and checks it is the
// version the catalog says.
func fetchListed(p api.ThemeCatalogPack, lichVersion string) (pack, error) {
	got, err := fetchPackAt(p.Repo, p.Commit, lichVersion)
	if err != nil {
		return pack{}, err
	}
	if got.manifest.version() != strings.TrimPrefix(p.Version, "v") {
		return pack{}, fmt.Errorf("%s at %s is version %s, not the %s the catalog lists", p.Name, p.Commit[:12], got.manifest.version(), p.Version)
	}
	return got, nil
}

// fetchPackAt reads the pack url holds at commit, fetching that commit alone.
func fetchPackAt(url, commit, lichVersion string) (pack, error) {
	remote, err := validateRemote(url)
	if err != nil {
		return pack{}, err
	}
	if !commitPattern.MatchString(commit) {
		return pack{}, fmt.Errorf("%q is not a full commit id", commit)
	}
	tmp, err := os.MkdirTemp("", "lich-theme-repo-")
	if err != nil {
		return pack{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	dir := filepath.Join(tmp, "repo")
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	if err := checkoutAt(ctx, remote, commit, dir); err != nil {
		return pack{}, err
	}
	return readPack(dir, remote, lichVersion)
}

// checkoutAt makes dir a checkout of url's commit and nothing more, with
// clone's guards: no ext:: helper, no credential prompt, a bounded wait.
func checkoutAt(ctx context.Context, url, commit, dir string) error {
	steps := [][]string{
		{"init", "--quiet", dir},
		{"-C", dir, "fetch", "--quiet", "--depth", "1", "--no-tags", "--", url, commit},
		{"-C", dir, "-c", "advice.detachedHead=false", "checkout", "--quiet", "FETCH_HEAD"},
	}
	for _, args := range steps {
		cmd := gitCmd(ctx, append([]string{"-c", "protocol.ext.allow=never", "-c", "credential.helper="}, args...))
		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("fetch %s: timed out after %s", url, cloneTimeout)
			}
			return fmt.Errorf("fetch %s at %s: %s", url, commit[:12], gitError(out, err))
		}
	}
	return nil
}

// gitCmd is git with args, hidden on Windows, never prompting, its wait bounded.
func gitCmd(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	winexec.Hide(cmd)
	cmd.Env = gitutil.NoPrompt(os.Environ())
	cmd.WaitDelay = cloneWait
	return cmd
}
