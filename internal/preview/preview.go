// Package preview serves a session's checkout, and its artifact directory,
// over loopback HTTP, so the session's browser can load a local file without a
// file: URL. A file: page follows its own links to any path on the machine,
// and the Chromium that loads it runs outside a confined session's sandbox; it
// also gets a null origin, which blocks module scripts and fetch. Served from an os.Root, the
// directory is the whole of what the page can reach.
package preview

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// A checkout's preview port is first tried one window above the dev-server
// port it reserves (internal/terminal/worktreeport.go, 24000-24999), so the two
// never collide, and the window stays below the ephemeral range Linux
// allocates outbound connections from (32768). A session's artifacts get the
// window above that, on a listener of their own: served from the checkout's
// origin, a page the agent wrote could fetch the checkout, a .env included.
const (
	portOffset         = 1000
	portBase           = 25000
	portCount          = 1000
	artifactPortOffset = 2000
	artifactPortBase   = 26000
)

const readHeaderTimeout = 10 * time.Second

var errNoCheckout = errors.New("this session has no checkout to serve files from")

// Service holds one listener per served directory, opened the first time a page from
// it is asked for and kept until Close.
type Service struct {
	mu      sync.Mutex
	servers map[string]*server
	listen  func(port int) (net.Listener, error)
}

type server struct {
	port int
	srv  *http.Server
}

// New returns a Service that binds the loopback interface.
func New() *Service {
	return &Service{servers: map[string]*server{}, listen: listenLoopback}
}

func listenLoopback(port int) (net.Listener, error) {
	return net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
}

// IsLocal reports whether raw names a local file, as a file: URL or an
// absolute path, rather than a URL the browser loads as is.
func IsLocal(raw string) bool {
	raw = strings.TrimSpace(raw)
	return strings.HasPrefix(strings.ToLower(raw), "file:") || filepath.IsAbs(raw)
}

// URL is the http URL that serves raw, a file: URL or an absolute path inside
// dir, from the checkout at dir whose dev-server port is devPort.
func (s *Service) URL(dir string, devPort int, raw string) (string, error) {
	if dir == "" || devPort == 0 {
		return "", errNoCheckout
	}
	path, err := localPath(raw)
	if err != nil {
		return "", err
	}
	dir = filepath.Clean(dir)
	rel, ok := within(dir, path)
	if !ok {
		return "", fmt.Errorf("%q is outside this session's checkout %q", path, dir)
	}
	port, err := s.serve(dir, portBase, devPort+portOffset)
	if err != nil {
		return "", err
	}
	return pageURL(port, rel), nil
}

// ArtifactURL is the http URL that serves raw when it names a file in dir, a
// session's artifact directory (internal/artifact), and ok is false when it
// does not, which leaves raw to the checkout. devPort is the session's
// dev-server port, 0 without one; it only places the listener.
func (s *Service) ArtifactURL(dir string, devPort int, raw string) (string, bool, error) {
	if dir == "" {
		return "", false, nil
	}
	path, err := localPath(raw)
	if err != nil {
		return "", false, err
	}
	dir = filepath.Clean(dir)
	rel, ok := within(dir, path)
	if !ok {
		return "", false, nil
	}
	first := artifactPortBase
	if devPort != 0 {
		first = devPort + artifactPortOffset
	}
	port, err := s.serve(dir, artifactPortBase, first)
	if err != nil {
		return "", true, err
	}
	return pageURL(port, rel), true, nil
}

// within is path relative to dir, and whether path is dir or inside it.
func within(dir, path string) (string, bool) {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func pageURL(port int, rel string) string {
	u := url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Path: "/"}
	if rel != "." {
		u.Path += filepath.ToSlash(rel)
	}
	return u.String()
}

// localPath is the absolute path raw names. A file: URL's path is
// percent-decoded, and on Windows loses the slash before its drive letter.
func localPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", fmt.Errorf("file url %q names another host", raw)
	}
	path := filepath.FromSlash(u.Path)
	if len(u.Path) > 2 && u.Path[0] == '/' && u.Path[2] == ':' {
		path = filepath.FromSlash(u.Path[1:])
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("file url %q has no absolute path", raw)
	}
	return filepath.Clean(path), nil
}

func (s *Service) serve(dir string, base, port int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv, ok := s.servers[dir]; ok {
		return srv.port, nil
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return 0, fmt.Errorf("%q is not a directory: %w", dir, err)
	}
	ln, err := s.listenFrom(base, port)
	if err != nil {
		return 0, fmt.Errorf("serve %q: %w", dir, err)
	}
	port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: loopbackHostOnly(port, rootHandler(dir)), ReadHeaderTimeout: readHeaderTimeout}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("preview server stopped", "dir", dir, "port", port, "err", err)
		}
	}()
	s.servers[dir] = &server{port: port, srv: srv}
	return port, nil
}

// loopbackHostOnly refuses a request whose Host is not one of the server's own
// loopback names. A hostname an attacker's page rebinds to 127.0.0.1 reaches
// this listener with the attacker's name in Host, and the browser would let
// that page read the checkout, a .env included.
func loopbackHostOnly(port int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, p, err := net.SplitHostPort(r.Host)
		if err != nil || p != strconv.Itoa(port) || (host != "127.0.0.1" && !strings.EqualFold(host, "localhost")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rootHandler serves dir through an os.Root opened for each request. One held
// for the server's life keeps the directory open, and Windows refuses to
// remove an open directory, so a worktree could not be deleted while its
// preview ran.
func rootHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		root, err := os.OpenRoot(dir)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = root.Close() }()
		http.FileServerFS(root.FS()).ServeHTTP(w, r)
	})
}

// listenFrom binds first, or the next free port after it in the window that
// starts at base, wrapping at its end: something else on the machine holding
// the preferred port costs the page its usual origin, not its preview.
func (s *Service) listenFrom(base, first int) (net.Listener, error) {
	var errs []error
	for offset := range portCount {
		port := base + (first-base+offset)%portCount
		ln, err := s.listen(port)
		if err == nil {
			return ln, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("no free port in %d-%d: %w", base, base+portCount-1, errors.Join(errs[0], errs[len(errs)-1]))
}

// Close stops every listener.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	for dir, srv := range s.servers {
		errs = append(errs, srv.srv.Close())
		delete(s.servers, dir)
	}
	return errors.Join(errs...)
}
