// Package artifact keeps the pages a session's agent writes to show the user,
// a report, a diagram, a mockup, one directory per session outside its
// checkout, so they never land in git. The session's browser opens them from
// there (internal/preview), whatever provider runs in it.
//
// A directory is named by the session's files key (store.SessionFilesKey),
// which a resume carries to the row's fresh id, so it lives exactly as long as
// the session: closing the card parks the row and keeps it, a restart and a
// resume find it again, and a path the agent already said keeps naming its
// page. Deleting the row deletes it (Purge), and a directory whose row is gone
// without lich seeing it go, a crash, is cleared at the next startup
// (PruneOrphans).
package artifact

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// EnvVar names the session's own artifact directory in its environment.
const EnvVar = "LICH_ARTIFACT_DIR"

// Dir is where a lich with this config directory keeps every session's
// artifacts.
func Dir(configDir string) string {
	return filepath.Join(configDir, "lich", "artifacts")
}

// SessionDir is the directory under root of the session whose files key is
// filesKey, "" for a key that is not a single path element.
func SessionDir(root, filesKey string) string {
	if root == "" || filesKey == "" || filesKey == "." || filesKey == ".." || filepath.Base(filesKey) != filesKey {
		return ""
	}
	return filepath.Join(root, filesKey)
}

type Service struct {
	root     string
	sessions Sessions
	emit     func(name string, data any)
	// live reports whether a row, open or parked, still holds a files key
	// (store.FilesKeyInUse).
	live func(filesKey string) bool
}

func New(root string, live func(filesKey string) bool) *Service {
	return &Service{root: root, live: live}
}

// Purge deletes the artifacts under one files key, its row being gone for
// good (store.SetSessionGone).
func (s *Service) Purge(filesKey string) {
	dir := SessionDir(s.root, filesKey)
	if dir == "" {
		return
	}
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("artifact: purge failed", "dir", dir, "err", err)
	}
}

// PruneOrphans deletes the directories no row holds the files key of. Startup
// only: no session has spawned yet, so none has its directory mounted.
func (s *Service) PruneOrphans() {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("artifact: prune could not read the artifacts dir", "dir", s.root, "err", err)
		}
		return
	}
	for _, entry := range entries {
		if s.live(entry.Name()) {
			continue
		}
		path := filepath.Join(s.root, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			slog.Warn("artifact: prune failed", "path", path, "err", err)
		}
	}
}
