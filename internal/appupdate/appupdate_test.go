package appupdate

import (
	"github.com/lichdotdev/lich/api"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseChecksum(t *testing.T) {
	data := []byte(
		"aaaa1111  lich-v0.8.0-linux-amd64\n" +
			"bbbb2222  lich-v0.8.0-darwin-arm64\n" +
			"cccc3333 *lich-v0.8.0-windows-amd64.exe\n", // binary-mode marker
	)
	tests := []struct {
		asset, want string
	}{
		{"lich-v0.8.0-darwin-arm64", "bbbb2222"},
		{"lich-v0.8.0-windows-amd64.exe", "cccc3333"},
		{"lich-v0.8.0-linux-amd64", "aaaa1111"},
		{"lich-v9.9.9-nope", ""},
	}
	for _, tc := range tests {
		if got := parseChecksum(data, tc.asset); got != tc.want {
			t.Errorf("parseChecksum(%q) = %q, want %q", tc.asset, got, tc.want)
		}
	}
}

func TestCanSelfApply(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "lich.exe")

	tests := []struct {
		name         string
		goos, goarch string
		exePath      string
		want         bool
	}{
		{"windows writable dir", "windows", "amd64", exe, true},
		{"windows arm64 has no installer", "windows", "arm64", exe, false},
		{"darwin never", "darwin", "arm64", exe, false},
		{"linux never", "linux", "amd64", exe, false},
		{"no exe path", "windows", "amd64", "", false},
		{"unwritable dir", "windows", "amd64", filepath.Join("/nonexistent-abc123", "lich.exe"), false},
		{"scoop install is scoop's", "windows", "amd64", scoopExe(t), false},
	}
	for _, tc := range tests {
		if got := canSelfApply(tc.goos, tc.goarch, tc.exePath); got != tc.want {
			t.Errorf("%s: canSelfApply = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// windowedExe returns a writable path with lich's window installed beside it,
// the way the Windows installer lays it out.
func windowedExe(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lich-native.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "lich.exe")
}

// scoopExe returns a writable path shaped like a Scoop install
// (<root>\apps\lich\current\lich.exe) with the window beside it, the way the
// manifest lays it out.
func scoopExe(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Scoop", "apps", "lich", "current")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lich-native.exe"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "lich.exe")
}

// bundleExe returns a writable path shaped like the cask install
// (/Applications/Lich.app/Contents/MacOS/lich), so the bundle is the only
// reason canSelfApply can refuse it.
func bundleExe(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Lich.app", "Contents", "MacOS")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "lich")
}

// packagedExe returns a path laid out the way the deb, rpm and AUR packages
// install lich: <root>/usr/bin/lich with the window beside it.
func packagedExe(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	touch(t, filepath.Join(root, "usr", "bin", "lich-native"))
	return filepath.Join(root, "usr", "bin", "lich")
}

// serveBody points a Service's releases endpoint at a test server.
func serveBody(t *testing.T, status int, body string) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Service{
		http:        srv.Client(),
		goos:        runtime.GOOS,
		goarch:      runtime.GOARCH,
		releasesURL: srv.URL,
		tagBase:     releaseTagBase,
	}
}

func TestStatus(t *testing.T) {
	// A writable exe dir so CanSelfApply is deterministic on the darwin/windows
	// branch; the test forces the version comparison, not the platform.
	s := serveBody(t, http.StatusOK, `[{"tag_name":"v0.8.0"}]`)
	s.exePath = filepath.Join(t.TempDir(), "lich")

	t.Run("update available", func(t *testing.T) {
		s.version = "0.7.0"
		got := s.status()
		if !got.UpdateAvailable {
			t.Fatalf("UpdateAvailable = false, want true (%+v)", got)
		}
		if got.CurrentVersion != "0.7.0" || got.LatestVersion != "0.8.0" {
			t.Fatalf("versions = %+v", got)
		}
		if got.ReleaseURL != "https://github.com/lichdotdev/lich/releases/tag/v0.8.0" {
			t.Fatalf("ReleaseURL = %q", got.ReleaseURL)
		}
		// CanSelfApply tracks the platform: only Windows amd64 on a writable dir.
		wantSelfApply := runtime.GOOS == "windows" && runtime.GOARCH == "amd64"
		if got.CanSelfApply != wantSelfApply {
			t.Fatalf("CanSelfApply = %v, want %v on %s", got.CanSelfApply, wantSelfApply, runtime.GOOS)
		}
	})

	t.Run("already latest", func(t *testing.T) {
		s.version = "0.8.0"
		if s.status().UpdateAvailable {
			t.Fatal("UpdateAvailable = true, want false when on the latest")
		}
	})

	t.Run("dev build never updates", func(t *testing.T) {
		s.version = "dev"
		got := s.status()
		if got.UpdateAvailable {
			t.Fatal("UpdateAvailable = true, want false for a dev build")
		}
		if got.CurrentVersion != "dev" {
			t.Fatalf("CurrentVersion = %q, want dev", got.CurrentVersion)
		}
	})

	// A build that only labels a release ("+sidebar.49") is that release, and
	// is offered the next one.
	t.Run("a labelled build of a release updates", func(t *testing.T) {
		s.version = "0.7.0+local.3"
		if !s.status().UpdateAvailable {
			t.Fatal("UpdateAvailable = false, want true for a build of 0.7.0 when 0.8.0 is out")
		}
	})

	t.Run("a git describe build never updates", func(t *testing.T) {
		s.version = "0.7.0-5-gabc123"
		if s.status().UpdateAvailable {
			t.Fatal("UpdateAvailable = true, want false for a build between tags")
		}
	})

	// The lookup runs at startup: a GitHub that answers 500 must leave the
	// readout empty rather than claim an update or block the boot.
	t.Run("a failed lookup reports nothing", func(t *testing.T) {
		failing := serveBody(t, http.StatusInternalServerError, "upstream is down")
		failing.version = "0.7.0"

		got := failing.status()

		if got.UpdateAvailable {
			t.Error("UpdateAvailable = true, want false when the lookup failed")
		}
		if got.LatestVersion != "" {
			t.Errorf("LatestVersion = %q, want empty", got.LatestVersion)
		}
		if got.ReleaseURL != "" {
			t.Errorf("ReleaseURL = %q, want empty — there is no release to point at", got.ReleaseURL)
		}
		if got.CurrentVersion != "0.7.0" {
			t.Errorf("CurrentVersion = %q, want the running build", got.CurrentVersion)
		}
	})
}

func TestInstallCommand(t *testing.T) {
	arch := "yay -S lich-bin" + restartChain
	cask := "brew tap lichdotdev/tap && brew upgrade --cask lichdotdev/tap/lich" + restartChain

	tests := []struct {
		name      string
		goos      string
		exePath   string
		osRelease string // written to a temp file; "" leaves osReleasePath missing
		want      string
	}{
		{"windows self-apply", "windows", "", "", ""},
		{"darwin outside a bundle", "darwin", "", "", ""},
		{"cask install", "darwin", bundleExe(t), "", cask},
		{"scoop install", "windows", scoopExe(t), "", "scoop update lich" + restartChainPwsh},
		{"scoop version dir", "windows", filepath.Join("C:", "scoop", "apps", "lich", "0.47.0", "lich.exe"), "", "scoop update lich" + restartChainPwsh},
		{"arch by ID", "linux", packagedExe(t), "ID=arch\n", arch},
		{"arch quoted ID", "linux", packagedExe(t), "ID=\"arch\"\n", arch},
		{"arch derivative via ID_LIKE", "linux", packagedExe(t), "ID=manjaro\nID_LIKE=arch\n", arch},
		{"debian uses install.sh", "linux", packagedExe(t), "ID=debian\n", installScript},
		{"fedora uses install.sh", "linux", packagedExe(t), "ID=fedora\nID_LIKE=\"rhel centos\"\n", installScript},
		{"missing os-release uses install.sh", "linux", packagedExe(t), "", installScript},
		{"linux tarball has no package to install", "linux", filepath.Join(t.TempDir(), "lich"), "ID=debian\n", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{goos: tc.goos, exePath: tc.exePath, osReleasePath: filepath.Join("/nonexistent-abc123", "os-release")}
			if tc.osRelease != "" {
				path := filepath.Join(t.TempDir(), "os-release")
				if err := os.WriteFile(path, []byte(tc.osRelease), 0o600); err != nil {
					t.Fatal(err)
				}
				s.osReleasePath = path
			}
			if got := s.installCommand(); got != tc.want {
				t.Errorf("installCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyRejectedWhenNotSelfApply(t *testing.T) {
	s := New("0.7.0", nil, nil)
	s.exePath = "" // forces canSelfApply false regardless of platform
	if _, err := s.Apply(t.Context(), api.None{}); err == nil {
		t.Fatal("Apply() = nil, want an error when self-apply is unsupported")
	}
}

// applyAsset is the installer the apply tests drive: the goos/goarch pair is
// pinned on the Service, so the test runs the same on every host.
const applyAsset = "lich-v0.8.0-windows-amd64-setup.exe"

// applyServer serves the release endpoints Apply hits: the releases list,
// checksums.txt, and the asset under assetStatus. 404 is the window between a
// pushed tag and a published installer.
func applyServer(t *testing.T, assetStatus int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases"):
			_, _ = io.WriteString(w, `[{"tag_name":"v0.8.0"}]`)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			_, _ = io.WriteString(w, "deadbeef  "+applyAsset+"\n")
		default:
			w.WriteHeader(assetStatus)
			_, _ = io.WriteString(w, "FAKEINSTALLER")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestApplyReportsProgress(t *testing.T) {
	body := "verified release asset"
	srv := windowsReleaseFixture(t, applyAsset, body)
	defer srv.Close()
	s := applyService(t, srv)
	s.downloadBase = srv.URL + "/"
	var steps []Progress
	s.emit = func(_ string, data any) { steps = append(steps, data.(Progress)) }
	if _, err := s.Apply(t.Context(), api.None{}); err != nil {
		t.Fatal(err)
	}
	if len(steps) < 3 {
		t.Fatalf("steps = %+v", steps)
	}
	if first := steps[0]; first != (Progress{Phase: phaseDownload, Received: 0, Total: int64(len(body))}) {
		t.Errorf("first = %+v", first)
	}
	if last := steps[len(steps)-1]; last != (Progress{Phase: phaseInstaller}) {
		t.Errorf("last = %+v", last)
	}
}

// applyService is a Service pinned to windows/amd64, the installer path driven
// off whatever host runs the suite. install records the staged installer.
func applyService(t *testing.T, srv *httptest.Server) *Service {
	t.Helper()
	return &Service{
		http:         srv.Client(),
		goos:         "windows",
		goarch:       "amd64",
		exePath:      filepath.Join(t.TempDir(), "lich.exe"),
		releasesURL:  srv.URL + "/releases",
		downloadBase: srv.URL + "/dl/",
		install:      func(string) error { return nil },
	}
}

// A tag is pushed before the release job finishes uploading, so the asset can
// be missing while the version lookup already reports the new release.
func TestApplyFailsWhenTheAssetIsNotPublishedYet(t *testing.T) {
	srv := applyServer(t, http.StatusNotFound)
	installed := false
	s := applyService(t, srv)
	s.install = func(string) error { installed = true; return nil }

	_, err := s.Apply(t.Context(), api.None{})

	if err == nil {
		t.Fatal("Apply() = nil, want an error when the asset download 404s")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want the status in it", err)
	}
	if installed {
		t.Error("Apply ran an installer for a 404 body")
	}
}

func TestFetchChecksum(t *testing.T) {
	t.Run("reads the asset's hash", func(t *testing.T) {
		s := serveBody(t, http.StatusOK, "aaaa  "+applyAsset+"\nbbbb  other\n")

		sum, err := s.fetchChecksum(s.releasesURL, applyAsset)
		if err != nil {
			t.Fatalf("fetchChecksum() error: %v", err)
		}
		if len(sum) != 2 || sum[0] != 0xaa || sum[1] != 0xaa {
			t.Fatalf("sum = %x, want aaaa", sum)
		}

		if _, err := s.fetchChecksum(s.releasesURL, "missing-asset"); err == nil {
			t.Fatal("fetchChecksum(missing) = nil error, want failure")
		}
	})

	t.Run("a non-200 is not a checksums file", func(t *testing.T) {
		s := serveBody(t, http.StatusNotFound, "Not Found")

		_, err := s.fetchChecksum(s.releasesURL, applyAsset)

		if err == nil {
			t.Fatal("fetchChecksum() = nil error, want failure on a non-200")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("error = %q, want the status in it", err)
		}
	})

	// checksums.txt is remote data: a truncated or corrupted line must fail the
	// update, never reach selfupdate as a half-decoded checksum.
	t.Run("a malformed hash is refused", func(t *testing.T) {
		s := serveBody(t, http.StatusOK, "zzzz  "+applyAsset+"\n")

		_, err := s.fetchChecksum(s.releasesURL, applyAsset)

		if err == nil {
			t.Fatal("fetchChecksum() = nil error, want failure on a non-hex hash")
		}
		if !strings.Contains(err.Error(), "checksums") {
			t.Errorf("error = %q, want the checksums context on it", err)
		}
	})
}

func TestNewResolvesExe(t *testing.T) {
	s := New("0.7.0", nil, nil)
	if s.version != "0.7.0" {
		t.Fatalf("version = %q", s.version)
	}
	// The asset download must not ride the short metadata timeout: a client
	// Timeout covers the whole body, and 5s cuts a multi-MiB binary mid-stream.
	if s.download == nil {
		t.Fatal("download client not set")
	}
	if s.download.Timeout <= s.http.Timeout {
		t.Fatalf("download timeout = %v, want longer than metadata %v", s.download.Timeout, s.http.Timeout)
	}
	if s.releasesURL != releasesURL {
		t.Fatalf("releasesURL = %q", s.releasesURL)
	}
	if s.goos != runtime.GOOS || s.goarch != runtime.GOARCH {
		t.Fatalf("platform = %s/%s, want this build's %s/%s", s.goos, s.goarch, runtime.GOOS, runtime.GOARCH)
	}
	exe, _ := os.Executable()
	if s.exePath != exe {
		t.Fatalf("exePath = %q, want %q", s.exePath, exe)
	}
}

// releaseList is a releases list as GitHub serves it, newest first: 0.10.0,
// 0.9.0 and 0.8.0, with caution under the banner of each version named.
func releaseList(caution ...string) string {
	entries := make([]string, 0, 3)
	for _, tag := range []string{"0.10.0", "0.9.0", "0.8.0"} {
		notes := `<img src=\"banner.png\" />\n\n\n### Fixed\n\n- A fix.\n`
		if slices.Contains(caution, tag) {
			notes = `<img src=\"banner.png\" />\n\n\n> [!CAUTION]\n> **Everyone: this closes a hole.** Update now.\n\n### Fixed\n\n- A fix.\n`
		}
		entries = append(entries, `{"tag_name":"v`+tag+`","body":"`+notes+`"}`)
	}
	return "[" + strings.Join(entries, ",") + "]"
}

func TestStatusRequired(t *testing.T) {
	tests := []struct {
		name    string
		version string
		caution []string
		want    bool
	}{
		{"the latest release is required", "0.7.0", []string{"0.10.0"}, true},
		// Skipping releases must not skip the one that was a security fix.
		{"a release in the middle is required", "0.7.0", []string{"0.9.0"}, true},
		{"none is required", "0.7.0", nil, false},
		{"the required release is the one running", "0.9.0", []string{"0.9.0"}, false},
		{"the required release is already behind", "0.9.0", []string{"0.8.0"}, false},
		{"on the latest", "0.10.0", []string{"0.10.0", "0.9.0"}, false},
		{"a dev build reports nothing", "dev", []string{"0.10.0"}, false},
		{"a build between tags reports nothing", "0.7.0-5-gabc123", []string{"0.10.0"}, false},
		{"a labelled build of a release is that release", "0.7.0+local.3", []string{"0.9.0"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := serveBody(t, http.StatusOK, releaseList(tc.caution...))
			s.version = tc.version
			got := s.status()
			if got.Required != tc.want {
				t.Fatalf("Required = %v, want %v (%+v)", got.Required, tc.want, got)
			}
			if tc.want && got.LatestVersion != "0.10.0" {
				t.Fatalf("LatestVersion = %q, want the newest release, the one to install", got.LatestVersion)
			}
		})
	}
}

func TestStatusRequiredIsFalseWhenTheLookupFails(t *testing.T) {
	s := serveBody(t, http.StatusInternalServerError, "upstream is down")
	s.version = "0.7.0"
	if s.status().Required {
		t.Fatal("Required = true, want false when the lookup failed")
	}
}

// The newest release is the highest version, wherever GitHub lists it: a patch
// of an older line published later must not be offered as the update.
func TestStatusLatestIsTheHighestVersion(t *testing.T) {
	s := serveBody(t, http.StatusOK, `[{"tag_name":"v0.8.1"},{"tag_name":"v0.9.0"},{"tag_name":"v0.8.0"}]`)
	s.version = "0.8.0"
	if got := s.status().LatestVersion; got != "0.9.0" {
		t.Fatalf("LatestVersion = %q, want 0.9.0", got)
	}
}

func TestRequiredUpdateRefusesWithTheWayToUpdate(t *testing.T) {
	s := serveBody(t, http.StatusOK, releaseList("0.9.0"))
	s.version = "0.7.0"
	s.goos, s.exePath, s.osReleasePath = "linux", packagedExe(t), filepath.Join(t.TempDir(), "os-release")

	err := s.RequiredUpdate()

	if err == nil {
		t.Fatal("RequiredUpdate() = nil, want a refusal while a required release is pending")
	}
	for _, want := range []string{"0.10.0", "0.7.0", "required security update", installScript, "https://github.com/lichdotdev/lich/releases/tag/v0.10.0"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want %q in it", err, want)
		}
	}
}

func TestRequiredErrorNamesTheRouteOfEachInstall(t *testing.T) {
	base := Status{CurrentVersion: "0.7.0", LatestVersion: "0.10.0", ReleaseURL: "https://example.test/v0.10.0"}
	installer, tarball := base, base
	installer.CanSelfApply = true
	if got := requiredError(installer).Error(); !strings.Contains(got, "update prompt in the lich window") || !strings.Contains(got, base.ReleaseURL) {
		t.Errorf("installer route = %q", got)
	}
	if got := requiredError(tarball).Error(); !strings.HasSuffix(got, "To update, download it from "+base.ReleaseURL) {
		t.Errorf("release page route = %q", got)
	}
}

func TestRequiredUpdateIsNilWithoutARequiredRelease(t *testing.T) {
	s := serveBody(t, http.StatusOK, releaseList())
	s.version = "0.7.0"
	if err := s.RequiredUpdate(); err != nil {
		t.Fatalf("RequiredUpdate() = %v, want nil", err)
	}
}

// The window's hourly poll is the lookup; opening sessions must not spend
// GitHub's hourly budget on top of it.
func TestRequiredUpdateAnswersFromTheLastLookup(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, releaseList("0.9.0"))
	}))
	t.Cleanup(srv.Close)
	s := &Service{http: srv.Client(), version: "0.7.0", releasesURL: srv.URL, tagBase: releaseTagBase}

	s.status()
	for range 3 {
		if err := s.RequiredUpdate(); err == nil {
			t.Fatal("RequiredUpdate() = nil, want the refusal the last lookup found")
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("lookups = %d, want 1: a fresh status answers without another request", got)
	}

	// With no window polling, the gate looks for itself once the answer is old.
	s.seen.Lock()
	s.checkedAt = time.Now().Add(-staleAfter - time.Minute)
	s.seen.Unlock()
	if err := s.RequiredUpdate(); err == nil {
		t.Fatal("RequiredUpdate() = nil after a stale lookup")
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("lookups = %d, want 2: a stale status is looked up again", got)
	}
}

// Opening a session in a dev build, a rig or a test run must not reach GitHub.
func TestRequiredUpdateLooksNothingUpForADevBuild(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, releaseList("0.10.0"))
	}))
	t.Cleanup(srv.Close)
	for _, version := range []string{"dev", "0.7.0-5-gabc123"} {
		s := &Service{http: srv.Client(), version: version, releasesURL: srv.URL, tagBase: releaseTagBase}
		if err := s.RequiredUpdate(); err != nil {
			t.Errorf("%s: RequiredUpdate() = %v, want nil", version, err)
		}
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("lookups = %d, want none", got)
	}
}
