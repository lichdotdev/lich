package extensions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

const (
	testRepo = "https://github.com/someone/fake-ext"
	testTag  = "v1.0.0"
	testID   = "fake"
)

type tarEntry struct {
	name string
	body []byte
	typ  byte
	link string
}

func tarball(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		h := &tar.Header{Name: e.name, Typeflag: typ, Linkname: e.link, Mode: 0o644}
		if typ == tar.TypeReg {
			h.Size = int64(len(e.body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.body[:h.Size]); err != nil {
			t.Fatal(err)
		}
	}
	if err := errors.Join(tw.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testExecutable() string {
	if runtime.GOOS == "windows" {
		return "ext.exe"
	}
	return "ext"
}

// extensionEntries is a well-formed extension whose executable holds exe.
func extensionEntries(t *testing.T, manifestID string, exe []byte) []tarEntry {
	t.Helper()
	raw, err := json.Marshal(validManifest(manifestID, testExecutable()))
	if err != nil {
		t.Fatal(err)
	}
	return []tarEntry{
		{name: extension.ManifestFile, body: raw},
		{name: testExecutable(), body: exe},
		{name: modeFile, body: []byte(modeEcho)},
	}
}

func selfBytes(t *testing.T) []byte {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func assetName(goos, goarch string) string {
	return testID + "_" + goos + "_" + goarch + assetSuffix
}

func hostAsset() string { return assetName(runtime.GOOS, runtime.GOARCH) }

func sumLine(name string, data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

// releaseServer publishes testTag with assets; a nil map is a repository with
// no release.
func releaseServer(t *testing.T, assets map[string][]byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/repos/someone/fake-ext/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		if assets == nil {
			http.NotFound(w, nil)
			return
		}
		rel := release{Tag: testTag}
		for name := range assets {
			rel.Assets = append(rel.Assets, asset{Name: name, URL: srv.URL + "/dl/" + name})
		}
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/dl/{name}", func(w http.ResponseWriter, r *http.Request) {
		body, ok := assets[r.PathValue("name")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	})
	return srv
}

// signed is assets plus the checksums.txt that vouches for them.
func signed(assets map[string][]byte) map[string][]byte {
	var sums strings.Builder
	for name, data := range assets {
		sums.WriteString(sumLine(name, data))
	}
	assets[checksumsAsset] = []byte(sums.String())
	return assets
}

func installer(t *testing.T, srv *httptest.Server) (*Service, *recorder, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "extensions")
	s, r := boot(t, dir)
	s.releaseAPI = srv.URL + "/repos/"
	return s, r, dir
}

func assertNothingInstalled(t *testing.T, s *Service, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("left behind %v", entries)
	}
	if list, _ := s.List(context.Background(), api.None{}); len(list) != 0 {
		t.Fatalf("List() = %v, want nothing", list)
	}
}

func TestInstallStartsTheLatestRelease(t *testing.T) {
	srv := releaseServer(t, signed(map[string][]byte{
		hostAsset():                tarball(t, extensionEntries(t, testID, selfBytes(t))),
		assetName("plan9", "mips"): []byte("not for this machine"),
	}))
	s, r, dir := installer(t, srv)

	got, err := s.Install(context.Background(), api.ExtensionInstallOptions{URL: testRepo + ".git"})
	if err != nil {
		t.Fatalf("Install() = %v", err)
	}
	if got.ID != testID || got.Source != testRepo || got.Release != testTag {
		t.Fatalf("Install() = %+v, want id %q from %s at %s", got, testID, testRepo, testTag)
	}
	waitStatus(t, r, statusRunning)
	if _, err := s.Send(context.Background(), sendOptions(t, testID, extension.SurfaceOpened.New(extension.SurfaceData{Surface: "main"}))); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	waitUntil(t, "the echo", func() bool { return slices.Contains(r.titles(t), "got surface.opened") })

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != testID {
		t.Fatalf("extensions dir = %v, %v; want only %s", entries, err, testID)
	}
	if again := load(filepath.Join(dir, testID), testID, "1.2.3"); again.source != (source{URL: testRepo, Release: testTag}) {
		t.Fatalf("a later discovery reads source %+v", again.source)
	}
}

func TestInstallRefusesARepositoryThatIsNotGitHub(t *testing.T) {
	for _, url := range []string{
		"http://github.com/someone/fake-ext", "https://gitlab.com/someone/fake-ext", "https://github.com/someone",
		"https://github.com/someone/fake-ext/tree/main", "git@github.com:someone/fake-ext.git",
		"https://github.com/someone/..", "https://github.com.evil.example/someone/fake-ext", "",
	} {
		s := &Service{}
		if _, err := s.Install(context.Background(), api.ExtensionInstallOptions{URL: url}); !errors.Is(err, errNotGitHub) {
			t.Errorf("Install(%q) = %v, want the GitHub-only refusal", url, err)
		}
	}
	for url, want := range map[string]string{
		testRepo: "fake-ext", testRepo + ".git": "fake-ext", " https://github.com/a-b/c.d_e ": "c.d_e",
	} {
		if _, repo, err := parseRepo(url); err != nil || repo != want {
			t.Errorf("parseRepo(%q) = %q, %v; want %q", url, repo, err, want)
		}
	}
}

func TestInstallFailsLeavingNothingBehind(t *testing.T) {
	good := func(t *testing.T) []byte { return tarball(t, extensionEntries(t, testID, []byte("x"))) }
	for name, tc := range map[string]struct {
		assets func(t *testing.T) map[string][]byte
		want   string
	}{
		"no release": {func(*testing.T) map[string][]byte { return nil }, "no published release"},
		"no build here": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{assetName("plan9", "mips"): good(t)})
		}, "no build for " + runtime.GOOS},
		"two builds here": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): good(t), "other_" + runtime.GOOS + "_" + runtime.GOARCH + assetSuffix: good(t)})
		}, "has 2 builds"},
		"bad asset id": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{"Bad_" + runtime.GOOS + "_" + runtime.GOARCH + assetSuffix: good(t)})
		}, "valid extension id"},
		"no checksums": {func(t *testing.T) map[string][]byte { return map[string][]byte{hostAsset(): good(t)} }, "has no checksums.txt"},
		"checksum missing": {func(t *testing.T) map[string][]byte {
			return map[string][]byte{hostAsset(): good(t), checksumsAsset: []byte("abc  other\n")}
		}, "no sha256 for"},
		"checksum mismatch": {func(t *testing.T) map[string][]byte {
			return map[string][]byte{hostAsset(): good(t), checksumsAsset: []byte(sumLine(hostAsset(), []byte("something else")))}
		}, "does not match its checksum"},
		"not gzip": {func(*testing.T) map[string][]byte { return signed(map[string][]byte{hostAsset(): []byte("plain")}) }, "not a gzip archive"},
		"parent path": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, append(extensionEntries(t, testID, []byte("x")), tarEntry{name: "../escaped", body: []byte("x")}))})
		}, "leaves the extension's directory"},
		"absolute path": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, append(extensionEntries(t, testID, []byte("x")), tarEntry{name: "/tmp/escaped", body: []byte("x")}))})
		}, "leaves the extension's directory"},
		"symlink": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, append(extensionEntries(t, testID, []byte("x")), tarEntry{name: "link", typ: tar.TypeSymlink, link: "/etc"}))})
		}, "links are not accepted"},
		"hard link": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, append(extensionEntries(t, testID, []byte("x")), tarEntry{name: "hard", typ: tar.TypeLink, link: modeFile}))})
		}, "links are not accepted"},
		"duplicate entry": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, append(extensionEntries(t, testID, []byte("x")), tarEntry{name: modeFile, body: []byte("x")}))})
		}, "entry \"mode\""},
		"manifest id differs": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, extensionEntries(t, "other", []byte("x")))})
		}, "differs from the release asset's"},
		"no manifest": {func(t *testing.T) map[string][]byte {
			return signed(map[string][]byte{hostAsset(): tarball(t, []tarEntry{{name: "dir/", typ: tar.TypeDir}, {name: "dir/x", body: []byte("x")}})})
		}, "is not valid"},
	} {
		t.Run(name, func(t *testing.T) {
			s, _, dir := installer(t, releaseServer(t, tc.assets(t)))
			_, err := s.Install(context.Background(), api.ExtensionInstallOptions{URL: testRepo})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Install() = %v, want an error naming %q", err, tc.want)
			}
			assertNothingInstalled(t, s, dir)
			if _, err := os.Lstat(filepath.Join(filepath.Dir(dir), "escaped")); err == nil {
				t.Fatal("an entry was written outside the extensions directory")
			}
		})
	}
}

func TestInstallRefusesAnIDAlreadyInstalled(t *testing.T) {
	srv := releaseServer(t, signed(map[string][]byte{hostAsset(): tarball(t, extensionEntries(t, testID, []byte("x")))}))
	dir := filepath.Join(t.TempDir(), "extensions")
	install(t, dir, testID, modeEcho)
	s, r := boot(t, dir)
	s.releaseAPI = srv.URL + "/repos/"
	waitStatus(t, r, statusRunning)

	_, err := s.Install(context.Background(), api.ExtensionInstallOptions{URL: testRepo})
	if err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Fatalf("Install() = %v, want the already-installed refusal", err)
	}
	if list, _ := s.List(context.Background(), api.None{}); len(list) != 1 || list[0].Source != "" {
		t.Fatalf("List() = %+v, want the hand-placed extension untouched", list)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("extensions dir = %v, want the one extension", entries)
	}
}

func TestRemoveStopsAndDeletes(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, testID, modeEcho)
	s, r := boot(t, dir)
	waitStatus(t, r, statusRunning)

	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the directory is still there: %v", err)
	}
	if r.lastStatus().Status != statusStopped {
		t.Fatalf("last status = %q, want stopped before the directory went", r.lastStatus().Status)
	}
	r.mu.Lock()
	removed := slices.Clone(r.removed)
	r.mu.Unlock()
	if !slices.Equal(removed, []string{testID}) {
		t.Fatalf("removed events = %v, want [%s]", removed, testID)
	}
	if list, _ := s.List(context.Background(), api.None{}); len(list) != 0 {
		t.Fatalf("List() = %v, want nothing", list)
	}
	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err == nil {
		t.Fatal("a second Remove() = nil, want unknown id refused")
	}
	if _, err := s.Restart(context.Background(), api.ExtensionIDOptions{ID: testID}); err == nil {
		t.Fatal("Restart() after Remove() = nil, want unknown id refused")
	}
}

func TestRemoveDeletesAnInvalidExtension(t *testing.T) {
	dir := t.TempDir()
	root := install(t, dir, testID, modeEcho, func(m *extension.Manifest) { m.Executable = "" })
	s, _ := boot(t, dir)
	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the directory is still there: %v", err)
	}
}

func TestDiscoverSkipsAnInstallInProgress(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, stagingPrefix+"123"), 0o755); err != nil {
		t.Fatal(err)
	}
	if found := discover(dir, "1.2.3"); len(found) != 0 {
		t.Fatalf("discover() = %d extensions, want the staging directory skipped", len(found))
	}
}

func TestRemoveUnlinksAnExtensionDevelopedInPlace(t *testing.T) {
	dir, work := t.TempDir(), t.TempDir()
	target := install(t, work, testID, modeEcho, func(m *extension.Manifest) { m.Executable = "" })
	if err := os.Symlink(target, filepath.Join(dir, testID)); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	s, _ := boot(t, dir)
	if _, err := s.Remove(context.Background(), api.ExtensionIDOptions{ID: testID}); err != nil {
		t.Fatalf("Remove() = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, testID)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the link is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, extension.ManifestFile)); err != nil {
		t.Fatalf("the link's target lost its files: %v", err)
	}
}
