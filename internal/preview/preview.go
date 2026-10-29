// Package preview serves a session's checkout over loopback HTTP, so the
// session's browser can load a local file without a file: URL. A file: page
// follows its own links to any path on the machine, and the Chromium that
// loads it runs outside a confined session's sandbox; it also gets a null
// origin, which blocks module scripts and fetch. Served from an os.Root, the
// checkout is the whole of what the page can reach.
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
// allocates outbound connections from (32768).
const (
	portOffset = 1000
	portBase   = 25000
	portCount  = 1000
)

const readHeaderTimeout = 10 * time.Second

var errNoCheckout = errors.New("this session has no checkout to serve files from")

// Service holds one listener per checkout, opened the first time a page from
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
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%q is outside this session's checkout %q", path, dir)
	}
	port, err := s.serve(dir, devPort+portOffset)
	if err != nil {
		return "", err
	}
	u := url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Path: "/"}
	if rel != "." {
		u.Path += filepath.ToSlash(rel)
	}
	return u.String(), nil
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

func (s *Service) serve(dir string, port int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv, ok := s.servers[dir]; ok {
		return srv.port, nil
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return 0, fmt.Errorf("checkout %q is not a directory: %w", dir, err)
	}
	ln, err := s.listenFrom(port)
	if err != nil {
		return 0, fmt.Errorf("serve checkout %q: %w", dir, err)
	}
	port = ln.Addr().(*net.TCPAddr).Port
	srv := &http.Server{Handler: rootHandler(dir), ReadHeaderTimeout: readHeaderTimeout}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("preview server stopped", "dir", dir, "port", port, "err", err)
		}
	}()
	s.servers[dir] = &server{port: port, srv: srv}
	return port, nil
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

// listenFrom binds first, or the next free port in the window after it,
// wrapping at its end: something else on the machine holding the preferred
// port costs the checkout its usual origin, not its preview.
func (s *Service) listenFrom(first int) (net.Listener, error) {
	var errs []error
	for offset := range portCount {
		port := portBase + (first-portBase+offset)%portCount
		ln, err := s.listen(port)
		if err == nil {
			return ln, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("no free port in %d-%d: %w", portBase, portBase+portCount-1, errors.Join(errs[0], errs[len(errs)-1]))
}

// Close stops every checkout's listener.
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
