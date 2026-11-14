package preview

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	s := New()
	s.listen = func(int) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s
}

func checkout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "site"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "site", "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func get(t *testing.T, u string) (int, string) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestURLServesAPathInsideTheCheckout(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	page := filepath.Join(dir, "site", "index.html")
	for _, raw := range []string{page, (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(page), "/")}).String()} {
		u, err := s.URL(dir, 24001, raw)
		if err != nil {
			t.Fatalf("URL(%q): %v", raw, err)
		}
		if !strings.HasPrefix(u, "http://127.0.0.1:") || !strings.HasSuffix(u, "/site/index.html") {
			t.Fatalf("URL(%q) = %q", raw, u)
		}
		if code, body := get(t, u); code != http.StatusOK || body != "<h1>hi</h1>" {
			t.Fatalf("GET %s = %d %q", u, code, body)
		}
	}
}

// A file the agent is editing: a stylesheet kept by Chrome's heuristic, a
// tenth of its age since it changed, would hide the edit on the next load.
func TestServedFileIsRevalidatedOnEveryLoad(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	u, err := s.URL(dir, 24001, filepath.Join(dir, "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}
}

func TestURLReusesOneListenerPerCheckout(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	first, err := s.URL(dir, 24001, dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.URL(dir, 24001, filepath.Join(dir, "site"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(second, first) {
		t.Fatalf("second checkout URL %q is not on %q", second, first)
	}
}

func TestURLRefusesAPathOutsideTheCheckout(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	outside := filepath.Join(filepath.Dir(dir), "elsewhere", "index.html")
	if _, err := s.URL(dir, 24001, outside); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("URL(outside) err = %v", err)
	}
	if _, err := s.URL("", 0, outside); err != errNoCheckout {
		t.Fatalf("URL without a checkout err = %v", err)
	}
	if _, err := s.URL(dir, 24001, "file://example.com/x"); err == nil {
		t.Fatal("a file url naming another host was served")
	}
}

func TestServedCheckoutDoesNotFollowASymlinkOut(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(dir, "leak.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	u, err := s.URL(dir, 24001, filepath.Join(dir, "leak.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if code, body := get(t, u); code == http.StatusOK || body == "key" {
		t.Fatalf("GET %s = %d %q, want the link refused", u, code, body)
	}
	if code, _ := get(t, strings.TrimSuffix(u, "leak.txt")+"../"+filepath.Base(filepath.Dir(secret))); code == http.StatusOK {
		t.Fatal("a dot-dot path left the checkout")
	}
}

// A page in the user's ordinary browser can rebind a hostname it controls to
// 127.0.0.1 and read the checkout through it; only the server's own loopback
// names are served.
func TestServedCheckoutRefusesAForeignHost(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	u, err := s.URL(dir, 24001, filepath.Join(dir, "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	port := parsed.Port()
	for host, want := range map[string]int{
		"127.0.0.1:" + port:     http.StatusOK,
		"localhost:" + port:     http.StatusOK,
		"LOCALHOST:" + port:     http.StatusOK,
		"evil.example:" + port:  http.StatusForbidden,
		"evil.example":          http.StatusForbidden,
		"127.0.0.1":             http.StatusForbidden,
		"127.0.0.1:1":           http.StatusForbidden,
		"127.0.0.1.evil.test:1": http.StatusForbidden,
	} {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Host %q: %v", host, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Host %q = %d, want %d", host, resp.StatusCode, want)
		}
	}
}

func TestIsLocal(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "a.html")
	for raw, want := range map[string]bool{
		abs:                   true,
		"file:///tmp/a.html":  true,
		"FILE:///tmp/a.html":  true,
		"https://example.com": false,
		"localhost:3000":      false,
		"javascript:alert(1)": false,
		"relative/index.html": false,
	} {
		if got := IsLocal(raw); got != want {
			t.Errorf("IsLocal(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestPreviewPortSitsAboveTheDevServerWindow(t *testing.T) {
	s := New()
	var asked int
	s.listen = func(port int) (net.Listener, error) {
		asked = port
		return net.Listen("tcp", "127.0.0.1:0")
	}
	t.Cleanup(func() { _ = s.Close() })
	dir := checkout(t)
	if _, err := s.URL(dir, 24007, dir); err != nil {
		t.Fatal(err)
	}
	if asked != 25007 {
		t.Fatalf("listened on %d, want 25007", asked)
	}
}

func TestURLMovesPastATakenPortAndWrapsTheWindow(t *testing.T) {
	for _, tc := range []struct{ dev, taken, want int }{
		{24007, 25007, 25008},
		{24999, 25999, 25000},
	} {
		s := New()
		var got int
		s.listen = func(port int) (net.Listener, error) {
			if port == tc.taken {
				return nil, errors.New("address already in use")
			}
			got = port
			return net.Listen("tcp", "127.0.0.1:0")
		}
		dir := checkout(t)
		if _, err := s.URL(dir, tc.dev, dir); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("dev %d with %d taken: listened on %d, want %d", tc.dev, tc.taken, got, tc.want)
		}
		_ = s.Close()
	}
}

func TestURLFailsOnlyWithTheWholeWindowTaken(t *testing.T) {
	s := New()
	tries := 0
	s.listen = func(int) (net.Listener, error) {
		tries++
		return nil, errors.New("address already in use")
	}
	dir := checkout(t)
	if _, err := s.URL(dir, 24000, dir); err == nil || !strings.Contains(err.Error(), "no free port") {
		t.Fatalf("err = %v", err)
	}
	if tries != portCount {
		t.Fatalf("tried %d ports, want %d", tries, portCount)
	}
}

func TestArtifactURLServesTheArtifactsFromAnOriginOfTheirOwn(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	artifacts := t.TempDir()
	if err := os.WriteFile(filepath.Join(artifacts, "report.html"), []byte("<h1>report</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	u, ok, err := s.ArtifactURL(artifacts, 24001, filepath.Join(artifacts, "report.html"))
	if err != nil || !ok {
		t.Fatalf("ArtifactURL = %q, %v, %v", u, ok, err)
	}
	if code, body := get(t, u); code != http.StatusOK || body != "<h1>report</h1>" {
		t.Fatalf("GET %s = %d %q", u, code, body)
	}
	page, err := s.URL(dir, 24001, filepath.Join(dir, "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	artifactURL, _ := url.Parse(u)
	checkoutURL, _ := url.Parse(page)
	if artifactURL.Host == checkoutURL.Host {
		t.Fatalf("artifacts and checkout share the origin %s", artifactURL.Host)
	}
}

func TestArtifactURLLeavesAPathOutsideTheArtifactsToTheCheckout(t *testing.T) {
	s := newTestService(t)
	dir := checkout(t)
	u, ok, err := s.ArtifactURL(t.TempDir(), 24001, filepath.Join(dir, "site", "index.html"))
	if ok || err != nil || u != "" {
		t.Fatalf("ArtifactURL = %q, %v, %v; want it left to the checkout", u, ok, err)
	}
	if u, ok, err := s.ArtifactURL("", 24001, filepath.Join(dir, "site", "index.html")); ok || err != nil || u != "" {
		t.Fatalf("ArtifactURL with no directory = %q, %v, %v", u, ok, err)
	}
}

func TestArtifactURLPrefersTheWindowAboveTheCheckoutsPreview(t *testing.T) {
	s := New()
	var asked []int
	s.listen = func(port int) (net.Listener, error) {
		asked = append(asked, port)
		return net.Listen("tcp", "127.0.0.1:0")
	}
	t.Cleanup(func() { _ = s.Close() })
	artifacts := t.TempDir()
	if _, _, err := s.ArtifactURL(artifacts, 24007, artifacts); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != 26007 {
		t.Fatalf("listened on %v, want [26007]", asked)
	}
	other := t.TempDir()
	asked = nil
	if _, _, err := s.ArtifactURL(other, 0, other); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || asked[0] != 26000 {
		t.Fatalf("listened on %v, want [26000]", asked)
	}
}
