package extensions

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	exti18n "github.com/lichdotdev/lich/extension/i18n"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/ghrelease"
)

const (
	releaseAPI      = "https://api.github.com/repos/"
	checksumsAsset  = "checksums.txt"
	assetSuffix     = ".tar.gz"
	metadataTimeout = 15 * time.Second
	// downloadTimeout is a hang stop for the archive, not a pace: a client
	// Timeout spans the whole body read.
	downloadTimeout = 5 * time.Minute
	// maxArchiveBytes bounds the compressed download; a Go binary for one
	// platform is tens of MiB.
	maxArchiveBytes = 256 << 20
	// sourceFile is lich's own record of where an installed extension came
	// from, beside its manifest.
	sourceFile = ".lich-source.json"
	// stagingPrefix names the work directory of an install in progress, which
	// discovery skips: an id can never start with a dot.
	stagingPrefix = ".installing-"
)

// repoPattern is the one URL shape PrepareFromGit takes. GitHub owners are letters,
// digits and dashes; repositories add dots and underscores.
var repoPattern = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?$`)

var errNotGitHub = errors.New("only GitHub repositories are supported: paste https://github.com/<owner>/<repo>")

// source is what sourceFile holds. Revoked is the reason the catalog gave
// for taking the repository down, kept here so the extension stays stopped
// when the catalog cannot be read: lich writes this file after unpacking, so
// one an archive ships is replaced.
type source struct {
	URL     string `json:"url"`
	Release string `json:"release"`
	Revoked string `json:"revoked,omitempty"`
}

type release struct {
	Tag    string  `json:"tag_name"`
	Assets []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// PrepareFromGit downloads a GitHub repository's extension for this
// platform, verifies and unpacks it, and stages it for CommitInstall: the
// release the catalog lists for a listed repository, the latest for another.
func (s *Service) PrepareFromGit(_ context.Context, opts api.ExtensionGitInstallOptions) (api.ExtensionInstallPreview, error) {
	owner, repo, err := parseRepo(opts.URL)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	url := "https://github.com/" + owner + "/" + repo
	cat := s.catalogNow()
	if reason := revokedReason(cat, url); reason != "" {
		return api.ExtensionInstallPreview{}, fmt.Errorf("%s/%s was %s; lich does not install it", owner, repo, revokedError(reason))
	}
	entry := listing(cat, url)
	rel, err := s.releaseFor(owner, repo, entry)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	id, archive, err := pickAsset(rel, s.goos, s.goarch)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	pinned, err := s.pinnedSum(entry)
	if err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	src := source{URL: url, Release: rel.Tag}
	return s.stageAs(id, src, func(stage string) error { return s.unpack(rel, archive, pinned, id, stage) })
}

// stageAs has fill lay the extension out in a fresh stage directory and checks
// it as discovery would, leaving it for CommitInstall. A failure at any step
// leaves nothing behind.
func (s *Service) stageAs(id string, src source, fill func(stage string) error) (api.ExtensionInstallPreview, error) {
	if err := s.checkFree(id); err != nil {
		return api.ExtensionInstallPreview{}, err
	}
	return s.stage(id, src, nil, fill)
}

// stage is stageAs for an id that may be installed: replaces is the extension
// CommitInstall swaps the staged copy for, nil for a new install.
func (s *Service) stage(id string, src source, replaces *installed, fill func(stage string) error) (api.ExtensionInstallPreview, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return api.ExtensionInstallPreview{}, fmt.Errorf("create the extensions directory: %w", err)
	}
	work, err := os.MkdirTemp(s.dir, stagingPrefix)
	if err != nil {
		return api.ExtensionInstallPreview{}, fmt.Errorf("create a work directory under %s: %w", s.dir, err)
	}
	stage := filepath.Join(work, id)
	m, err := s.fillStage(stage, id, src, fill)
	if err != nil {
		_ = os.RemoveAll(work)
		return api.ExtensionInstallPreview{}, err
	}
	token, err := s.hold(id, work, replaces)
	if err != nil {
		_ = os.RemoveAll(work)
		return api.ExtensionInstallPreview{}, err
	}
	return api.ExtensionInstallPreview{
		Token: token, ID: id, Name: cmp.Or(m.Name, id), Version: m.Version,
		Source: src.URL, Release: src.Release, Permissions: permissionNames(m),
	}, nil
}

func (s *Service) fillStage(stage, id string, src source, fill func(stage string) error) (extension.Manifest, error) {
	if err := fill(stage); err != nil {
		return extension.Manifest{}, err
	}
	m, err := checkStage(stage, id, s.lichVersion)
	if err != nil {
		return extension.Manifest{}, err
	}
	return m, writeSource(stage, src)
}

func parseRepo(url string) (owner, repo string, err error) {
	m := repoPattern.FindStringSubmatch(strings.TrimSpace(url))
	if m == nil || m[2] == "." || m[2] == ".." {
		return "", "", errNotGitHub
	}
	return m[1], m[2], nil
}

// releaseFor is the release the catalog lists for a listed repository, and
// GitHub's latest for any other.
func (s *Service) releaseFor(owner, repo string, entry *extension.CatalogEntry) (release, error) {
	if entry == nil {
		return s.latestRelease(owner, repo)
	}
	return s.taggedRelease(owner, repo, entry.Tag)
}

func (s *Service) latestRelease(owner, repo string) (release, error) {
	missing := fmt.Errorf("%s/%s has no published release, or is not a public repository: check the URL, or ask its author to publish a release", owner, repo)
	return s.readRelease(owner, repo, "latest", "the latest release", missing)
}

func (s *Service) taggedRelease(owner, repo, tag string) (release, error) {
	missing := fmt.Errorf("%s/%s has no release %s, the one the lich catalog lists: try again later, or tell the extension's author", owner, repo, tag)
	return s.readRelease(owner, repo, "tags/"+neturl.PathEscape(tag), "release "+tag, missing)
}

func (s *Service) readRelease(owner, repo, which, what string, missing error) (release, error) {
	var body bytes.Buffer
	url := s.releaseAPI + owner + "/" + repo + "/releases/" + which
	status, err := fetch(s.metadata, url, ghrelease.ListLimit, &body)
	if status == http.StatusNotFound {
		return release{}, missing
	}
	if err != nil {
		return release{}, fmt.Errorf("look up %s of %s/%s: %w; try again later", what, owner, repo, err)
	}
	var rel release
	if err := json.Unmarshal(body.Bytes(), &rel); err != nil || rel.Tag == "" {
		return release{}, fmt.Errorf("GitHub's answer for %s/%s is not a release; try again later", owner, repo)
	}
	return rel, nil
}

// pickAsset finds the one archive for goos/goarch and the id its name gives.
func pickAsset(rel release, goos, goarch string) (string, asset, error) {
	suffix := "_" + goos + "_" + goarch + assetSuffix
	var found []asset
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, suffix) {
			found = append(found, a)
		}
	}
	if len(found) == 0 {
		return "", asset{}, fmt.Errorf("release %s has no build for %s/%s: ask the extension's author to publish <id>%s", rel.Tag, goos, goarch, suffix)
	}
	if len(found) > 1 {
		return "", asset{}, fmt.Errorf("release %s has %d builds for %s/%s and lich cannot tell which to install: ask the extension's author to publish one", rel.Tag, len(found), goos, goarch)
	}
	id := strings.TrimSuffix(found[0].Name, suffix)
	if !idPattern.MatchString(id) {
		return "", asset{}, fmt.Errorf("release asset %s does not start with a valid extension id: ask the extension's author to rename it", found[0].Name)
	}
	return id, found[0], nil
}

func (s *Service) checkFree(id string) error {
	s.mu.Lock()
	known := s.find(id) != nil
	s.mu.Unlock()
	_, statErr := os.Lstat(filepath.Join(s.dir, id))
	if known || !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("an extension %q is already installed: remove it first to install this one", id)
	}
	return nil
}

// pinnedSum is the sha256 the catalog recorded for this platform's archive
// of a listed repository, nil for an unlisted one.
func (s *Service) pinnedSum(entry *extension.CatalogEntry) ([]byte, error) {
	if entry == nil {
		return nil, nil
	}
	sum, err := hex.DecodeString(entry.Builds[s.goos+"_"+s.goarch])
	if err != nil || len(sum) != sha256.Size {
		return nil, fmt.Errorf("the lich catalog lists no build of %s %s for %s/%s: it does not install here yet", entry.ID, entry.Version, s.goos, s.goarch)
	}
	return sum, nil
}

// unpack downloads the archive into stage after checking it against the
// release's checksums and, for a listed repository, the sha256 pinned.
func (s *Service) unpack(rel release, archive asset, pinned []byte, id, stage string) error {
	want, err := s.checksum(rel, archive.Name)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(stage), "archive-")
	if err != nil {
		return fmt.Errorf("create the download file: %w", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := fetch(s.download, archive.URL, maxArchiveBytes, io.MultiWriter(file, hash)); err != nil {
		return fmt.Errorf("download %s: %w; try again later", archive.Name, err)
	}
	if !bytes.Equal(hash.Sum(nil), want) {
		return fmt.Errorf("%s does not match its checksum in %s: the download is corrupt or was tampered with; do not install it, and tell the extension's author", archive.Name, checksumsAsset)
	}
	if pinned != nil && !bytes.Equal(hash.Sum(nil), pinned) {
		return fmt.Errorf("%s of release %s differs from the archive the lich catalog recorded: it was replaced since; do not install it, and tell the extension's author", archive.Name, rel.Tag)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("read the download back: %w", err)
	}
	if err := extract(file, stage); err != nil {
		return fmt.Errorf("unpack %s: %w; ask the extension's author to fix the release", archive.Name, err)
	}
	return nil
}

func (s *Service) checksum(rel release, name string) ([]byte, error) {
	var sums *asset
	for i := range rel.Assets {
		if rel.Assets[i].Name == checksumsAsset {
			sums = &rel.Assets[i]
		}
	}
	if sums == nil {
		return nil, fmt.Errorf("release %s has no %s, so its download cannot be checked: ask the extension's author to publish one", rel.Tag, checksumsAsset)
	}
	var body bytes.Buffer
	if _, err := fetch(s.metadata, sums.URL, ghrelease.BodyLimit, &body); err != nil {
		return nil, fmt.Errorf("download %s: %w; try again later", checksumsAsset, err)
	}
	sum, err := hex.DecodeString(parseChecksum(body.String(), name))
	if err != nil || len(sum) != sha256.Size {
		return nil, fmt.Errorf("%s of release %s has no sha256 for %s: ask the extension's author to fix it", checksumsAsset, rel.Tag, name)
	}
	return sum, nil
}

// parseChecksum finds name's hash in sha256sum-format lines ("<hex>  <name>").
func parseChecksum(data, name string) string {
	for line := range strings.SplitSeq(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == name {
			return fields[0]
		}
	}
	return ""
}

// checkStage runs discovery's validation on the unpacked directory and makes
// the executable runnable: tar modes are the author's, and lich does not trust
// them to be right.
func checkStage(stage, id, lichVersion string) (extension.Manifest, error) {
	m, exe, err := readManifest(stage, id, lichVersion)
	if err != nil && m.ID != "" && m.ID != id {
		return m, fmt.Errorf("the manifest's id %q differs from the release asset's %q: ask the extension's author to make them match", m.ID, id)
	}
	if err != nil {
		return m, fmt.Errorf("the extension's %s is not valid: %w", extension.ManifestFile, err)
	}
	if _, err := readLocales(stage, m); err != nil {
		return m, fmt.Errorf("the extension's %s are not valid: %w", exti18n.LocalesDir, err)
	}
	if _, err := readIcons(stage, m); err != nil {
		return m, fmt.Errorf("the extension's %s is not valid: %w", extension.ManifestFile, err)
	}
	if err := os.Chmod(exe, 0o755); err != nil {
		return m, fmt.Errorf("make the executable runnable: %w", err)
	}
	return m, nil
}

func writeSource(stage string, src source) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, sourceFile), raw, 0o644); err != nil {
		return fmt.Errorf("record where the extension came from: %w", err)
	}
	return nil
}

// readSource is the empty source for an extension placed by hand.
func readSource(root string) source {
	var src source
	raw, err := os.ReadFile(filepath.Join(root, sourceFile))
	if err == nil && json.Unmarshal(raw, &src) != nil {
		slog.Warn("extensions: unreadable source record", "dir", root)
	}
	return src
}

// place moves stage into the extensions directory and starts it. The rename
// is the commit: before it nothing is installed, after it the extension is.
func (s *Service) place(id, stage string) (api.Extension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return api.Extension{}, errors.New("lich is shutting down: install the extension after it starts again")
	}
	root := filepath.Join(s.dir, id)
	if err := os.Rename(stage, root); err != nil {
		return api.Extension{}, fmt.Errorf("move the extension into %s: %w", root, err)
	}
	x := load(root, id, s.lichVersion)
	s.revokeLoaded(x)
	s.all = append(s.all, x)
	if x.invalid == nil {
		s.launch(x)
	}
	return x.view(), nil
}

// Remove stops an extension and deletes its directory. A directory that is a
// symlink, an extension developed in place, loses the link and keeps its
// target. Its data directory stays, for DeleteData or the next install of the
// same id.
func (s *Service) Remove(ctx context.Context, opts api.ExtensionIDOptions) (api.None, error) {
	s.mu.Lock()
	x := s.find(opts.ID)
	s.mu.Unlock()
	if x == nil {
		return api.None{}, fmt.Errorf("no extension %q", opts.ID)
	}
	x.restartMu.Lock()
	defer x.restartMu.Unlock()
	s.mu.Lock()
	stop, done := x.stop, x.done
	s.mu.Unlock()
	if stop != nil {
		stop()
		select {
		case <-done:
		case <-ctx.Done():
			return api.None{}, ctx.Err()
		}
	}
	if err := afterExit(func() error { return os.RemoveAll(x.root) }); err != nil {
		return api.None{}, fmt.Errorf("delete %s: %w; delete it by hand", x.root, err)
	}
	s.mu.Lock()
	s.all = removeFrom(s.all, x)
	s.mu.Unlock()
	events.Send(s.emit, api.ExtensionRemovedEvent, api.ExtensionIDOptions{ID: x.id})
	return api.None{}, nil
}

// DeleteData deletes the data directory of an extension that is not
// installed. installMu keeps an install of the same id from starting while it
// goes.
func (s *Service) DeleteData(_ context.Context, opts api.ExtensionIDOptions) (api.None, error) {
	if !idPattern.MatchString(opts.ID) {
		return api.None{}, fmt.Errorf("%q is not an extension id", opts.ID)
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	if s.checkFree(opts.ID) != nil {
		return api.None{}, fmt.Errorf("extension %q is installed: remove it first, then delete its data", opts.ID)
	}
	data := filepath.Join(s.dataDir, opts.ID)
	if err := os.RemoveAll(data); err != nil {
		return api.None{}, fmt.Errorf("delete %s: %w; delete it by hand", data, err)
	}
	return api.None{}, nil
}

func removeFrom(all []*installed, x *installed) []*installed {
	out := make([]*installed, 0, len(all))
	for _, y := range all {
		if y != x {
			out = append(out, y)
		}
	}
	return out
}

// fetch copies url's body into w, refusing one over limit. The status is
// returned apart so a caller can tell a missing release from a failed read.
func fetch(client *http.Client, url string, limit int64, w io.Writer) (int, error) {
	resp, err := ghrelease.Get(client, url)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return resp.StatusCode, err
	}
	if n > limit {
		return resp.StatusCode, fmt.Errorf("larger than the %d MiB lich accepts", limit>>20)
	}
	return resp.StatusCode, nil
}
