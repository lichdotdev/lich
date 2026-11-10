package artifact

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/prompt"
)

// watchEvery is how often RunWatch reads the open sessions' folders. A read is
// one directory listing per open session, so a new page reaches the window's
// Artifacts tab within this much without a file-watching dependency.
const watchEvery = 2 * time.Second

// indexPage is the page a folder artifact opens at; a folder without one is
// not listed.
const indexPage = "index.html"

// maxCopies bounds how many copies of one name land side by side in a
// session's folder (the name and its -2… suffixes).
const maxCopies = 999

// Sessions is what the lists and copies read the sessions through
// (internal/store). Startup wiring, see SetSessions.
type Sessions struct {
	// Open maps every open session to its files key.
	Open  func() (map[string]string, error)
	Key   func(sessionID string) string
	Label func(sessionID string) string
	Lang  func() prompt.Lang
	// Show loads the page at path, in the artifact folder of session from,
	// in session's browser (main.go: the preview server and internal/browser).
	Show func(session, from, path string) error
}

// SetSessions wires the sessions and where ArtifactsEvent goes. Startup
// wiring, called before anything serves.
func (s *Service) SetSessions(sessions Sessions, emit func(name string, data any)) {
	s.sessions, s.emit = sessions, emit
}

// List is every open session's artifacts, for a window that just attached.
func (s *Service) List(_ context.Context, _ api.None) ([]api.SessionArtifacts, error) {
	open, err := s.sessions.Open()
	if err != nil {
		return nil, err
	}
	out := make([]api.SessionArtifacts, 0, len(open))
	for id, key := range open {
		out = append(out, api.SessionArtifacts{SessionID: id, Files: listDir(SessionDir(s.root, key))})
	}
	slices.SortFunc(out, func(a, b api.SessionArtifacts) int { return strings.Compare(a.SessionID, b.SessionID) })
	return out, nil
}

// RunWatch emits ArtifactsEvent for every open session whose list changed
// since the last pass, and an empty one for a session no longer open. It
// never returns.
func (s *Service) RunWatch() {
	seen := map[string][]api.Artifact{}
	ticker := time.NewTicker(watchEvery)
	defer ticker.Stop()
	for range ticker.C {
		s.watchOnce(seen)
	}
}

func (s *Service) watchOnce(seen map[string][]api.Artifact) {
	open, err := s.sessions.Open()
	if err != nil {
		slog.Warn("artifact: list open sessions", "err", err)
		return
	}
	for id, key := range open {
		files := listDir(SessionDir(s.root, key))
		last, known := seen[id]
		if (known || len(files) > 0) && !slices.Equal(last, files) {
			events.Send(s.emit, api.ArtifactsEvent, api.SessionArtifacts{SessionID: id, Files: files})
		}
		seen[id] = files
	}
	for id, last := range seen {
		if _, ok := open[id]; ok {
			continue
		}
		if len(last) > 0 {
			events.Send(s.emit, api.ArtifactsEvent, api.SessionArtifacts{SessionID: id, Files: []api.Artifact{}})
		}
		delete(seen, id)
	}
}

// listDir is the artifacts in dir, newest first. A symlink is left out: the
// page it names may be anywhere on the machine, and the preview server would
// refuse it anyway (internal/preview serves from an os.Root).
func listDir(dir string) []api.Artifact {
	files := []api.Artifact{}
	if dir == "" {
		return files
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("artifact: read folder", "dir", dir, "err", err)
		}
		return files
	}
	for _, entry := range entries {
		if a, ok := artifactOf(dir, entry); ok {
			files = append(files, a)
		}
	}
	slices.SortFunc(files, func(a, b api.Artifact) int {
		return cmp.Or(cmp.Compare(b.Modified, a.Modified), strings.Compare(a.Name, b.Name))
	})
	return files
}

func artifactOf(dir string, entry fs.DirEntry) (api.Artifact, bool) {
	name := entry.Name()
	if strings.HasPrefix(name, ".") || entry.Type()&fs.ModeSymlink != 0 {
		return api.Artifact{}, false
	}
	path := filepath.Join(dir, name)
	if entry.IsDir() {
		path = filepath.Join(path, indexPage)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return api.Artifact{}, false
	}
	return api.Artifact{Name: name, Path: path, Modified: info.ModTime().UnixMilli()}, true
}

// Attach puts From's artifact Name where SessionID can read it. The same
// session gets the artifact's own path; another gets a copy in its own folder,
// since a confined session cannot read the folder of the session beside it,
// and the copy outlives From. The source is read through an os.Root, so a
// link inside From's folder cannot copy a file from outside it.
func (s *Service) Attach(_ context.Context, opts api.AttachArtifactOptions) (api.DropAttachment, error) {
	fromKey, toKey := s.sessions.Key(opts.From), s.sessions.Key(opts.SessionID)
	from, to := SessionDir(s.root, fromKey), SessionDir(s.root, toKey)
	if from == "" || to == "" {
		return api.DropAttachment{}, errors.New("no artifact folder for these sessions")
	}
	listed, ok := find(from, opts.Name)
	if !ok {
		return api.DropAttachment{}, fmt.Errorf("artifact %q is not in the folder of session %q", opts.Name, opts.From)
	}
	if fromKey == toKey {
		return api.DropAttachment{Path: listed.Path}, nil
	}
	src, err := os.OpenRoot(from)
	if err != nil {
		return api.DropAttachment{}, fmt.Errorf("open the artifacts of session %q: %w", opts.From, err)
	}
	defer func() { _ = src.Close() }()
	if err := os.MkdirAll(to, 0o755); err != nil {
		return api.DropAttachment{}, fmt.Errorf("create the artifact folder: %w", err)
	}
	dst, err := uniquePath(to, opts.Name)
	if err != nil {
		return api.DropAttachment{}, err
	}
	path, err := copyArtifact(src, opts.Name, dst, listed.Path != filepath.Join(from, opts.Name))
	if err != nil {
		return api.DropAttachment{}, err
	}
	notice := fmt.Sprintf(prompt.For(s.sessions.Lang()).ArtifactCopyNotice, opts.Name, s.sessions.Label(opts.From))
	return api.DropAttachment{Path: path, Notice: notice}, nil
}

// Open loads From's artifact Name in SessionID's browser.
func (s *Service) Open(_ context.Context, opts api.AttachArtifactOptions) (api.None, error) {
	listed, ok := find(SessionDir(s.root, s.sessions.Key(opts.From)), opts.Name)
	if !ok {
		return api.None{}, fmt.Errorf("artifact %q is not in the folder of session %q", opts.Name, opts.From)
	}
	return api.None{}, s.sessions.Show(opts.SessionID, opts.From, listed.Path)
}

// find is the artifact named name in dir, by the rules listDir lists with.
func find(dir, name string) (api.Artifact, bool) {
	for _, a := range listDir(dir) {
		if a.Name == name {
			return a, true
		}
	}
	return api.Artifact{}, false
}

// copyArtifact copies name from src to dst and answers with the path to open:
// the copy itself, or its index page for a folder.
func copyArtifact(src *os.Root, name, dst string, dir bool) (string, error) {
	if dir {
		sub, err := fs.Sub(src.FS(), name)
		if err != nil {
			return "", err
		}
		if err := os.CopyFS(dst, sub); err != nil {
			_ = os.RemoveAll(dst)
			return "", fmt.Errorf("copy artifact %q: %w", name, err)
		}
		return filepath.Join(dst, indexPage), nil
	}
	in, err := src.Open(name)
	if err != nil {
		return "", fmt.Errorf("open artifact %q: %w", name, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create the copy of %q: %w", name, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return "", fmt.Errorf("copy artifact %q: %w", name, err)
	}
	return dst, out.Close()
}

// uniquePath is dir/name, or dir/name-N.ext for the first N not taken: an
// earlier copy's path may still sit in a prompt.
func uniquePath(dir, name string) (string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 1; n <= maxCopies; n++ {
		candidate := name
		if n > 1 {
			candidate = stem + "-" + strconv.Itoa(n) + ext
		}
		path := filepath.Join(dir, candidate)
		if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%d copies of %q already exist", maxCopies, name)
}
