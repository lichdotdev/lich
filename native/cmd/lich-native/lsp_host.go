package main

// The language servers the editors use while Vim is off: one per server and
// project, started at the first file that wants it and kept for the window's
// life, as godemirror's lsp package runs them.

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/godemirror/lsp"
	"github.com/lichdotdev/godemirror/lsp/servers"
)

// lspStartTimeout bounds a server's start and its initialize handshake.
const lspStartTimeout = 30 * time.Second

type lspHost struct {
	invalidate func()
	mu         sync.Mutex
	running    map[lspKey]*lspServer
}

type lspKey struct{ root, name string }

// lspServer is one server of a project: starting while neither client nor
// err is set, failed once err is, with reason the first line it wrote to
// its standard error.
type lspServer struct {
	mu     sync.Mutex
	client *lsp.Client
	err    error
	reason string
}

func newLspHost(invalidate func()) *lspHost {
	return &lspHost{invalidate: invalidate, running: map[lspKey]*lspServer{}}
}

// state is the server's client, nil while it starts or once it failed, and
// its failure.
func (s *lspServer) state() (*lsp.Client, error, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client, s.err, s.reason
}

// server is s running for the project at root, started now when it is not
// yet; command is where s's program was found.
func (h *lspHost) server(root string, s servers.Server, command []string) *lspServer {
	key := lspKey{root, s.Name}
	h.mu.Lock()
	defer h.mu.Unlock()
	if srv := h.running[key]; srv != nil {
		return srv
	}
	srv := new(lspServer)
	h.running[key] = srv
	go h.start(srv, root, s, command)
	return srv
}

// forget drops a failed server, so the next file that wants it starts it
// again: after an install, or once the person fixed what failed it.
func (h *lspHost) forget(root, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if srv := h.running[lspKey{root, name}]; srv != nil {
		if _, err, _ := srv.state(); err != nil {
			delete(h.running, lspKey{root, name})
		}
	}
}

func (h *lspHost) start(srv *lspServer, root string, s servers.Server, command []string) {
	stderr := new(lspFirstLine)
	ctx, cancel := context.WithTimeout(context.Background(), lspStartTimeout)
	defer cancel()
	c, err := lsp.Start(ctx, lsp.Config{Command: command, RootDir: root, InitializationOptions: s.InitOptions,
		Settings: s.Settings, Stderr: stderr, Env: []string{"PATH=" + lspPath()}})
	srv.mu.Lock()
	srv.client, srv.err, srv.reason = c, err, stderr.String()
	srv.mu.Unlock()
	h.invalidate()
	if err != nil {
		return
	}
	<-c.Done()
	srv.mu.Lock()
	srv.client, srv.err, srv.reason = nil, errors.New("exited"), stderr.String()
	srv.mu.Unlock()
	h.invalidate()
}

// lspCommand is s's command with its program found on the PATH or among the
// servers lich installed, nil when it is neither.
func lspCommand(s servers.Server) []string {
	if len(s.Command) == 0 {
		return nil
	}
	if p, err := lspLookPath(s.Command[0]); err == nil {
		return append([]string{p}, s.Command[1:]...)
	}
	return nil
}

func lspLookPath(name string) (string, error) {
	for _, dir := range filepath.SplitList(lspPath()) {
		p := filepath.Join(dir, name)
		if _, err := exec.LookPath(p); err == nil {
			return p, nil
		}
	}
	return "", exec.ErrNotFound
}

// lspPath is the PATH with the folders of the servers lich installed after
// it: what the person installed comes first.
func lspPath() string {
	dirs := []string{os.Getenv("PATH")}
	if root, err := lspInstallRoot(); err == nil {
		bins, _ := filepath.Glob(filepath.Join(root, "*", "bin"))
		mods, _ := filepath.Glob(filepath.Join(root, "*", "node_modules", ".bin"))
		venvs, _ := filepath.Glob(filepath.Join(root, "*", "venv", "bin"))
		dirs = append(append(append(dirs, bins...), mods...), venvs...)
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// lspInstallRoot is where lich installs servers, a folder each.
func lspInstallRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "lich", "lsp"), nil
}

// lspFirstLine keeps the first line written to it: a server's stderr says
// why it failed there, and its noise after.
type lspFirstLine struct {
	mu   sync.Mutex
	line strings.Builder
	done bool
}

func (w *lspFirstLine) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range p {
		if w.done {
			break
		}
		if b == '\n' {
			w.done = w.line.Len() > 0
			continue
		}
		w.line.WriteByte(b)
	}
	return len(p), nil
}

func (w *lspFirstLine) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.TrimSpace(w.line.String())
}
