package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

// InstallFromDir copies a folder holding manifest.json and the executable at
// its root into the extensions directory and starts the copy. The folder is
// left as it was.
func (s *Service) InstallFromDir(_ context.Context, opts api.ExtensionDirInstallOptions) (api.Extension, error) {
	from, err := filepath.Abs(opts.Dir)
	if err != nil {
		return api.Extension{}, fmt.Errorf("resolve %s: %w", opts.Dir, err)
	}
	if info, err := os.Lstat(from); err != nil || !info.IsDir() {
		return api.Extension{}, fmt.Errorf("%s is not a folder: pick the folder that holds the extension's %s", opts.Dir, extension.ManifestFile)
	}
	id, err := manifestID(from)
	if err != nil {
		return api.Extension{}, err
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	return s.installAs(id, source{URL: from}, func(stage string) error {
		if err := copyTree(from, stage); err != nil {
			return fmt.Errorf("copy %s: %w", from, err)
		}
		return nil
	})
}

// manifestID reads the id the folder's manifest gives, checked before it
// names a path: the full validation runs on the copy.
func manifestID(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, extension.ManifestFile))
	if err != nil {
		return "", fmt.Errorf("%s has no readable %s at its root: pick the extension's own folder", dir, extension.ManifestFile)
	}
	var m extension.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", fmt.Errorf("parse %s: %w", extension.ManifestFile, err)
	}
	if !idPattern.MatchString(m.ID) {
		return "", fmt.Errorf("id %q must be lowercase letters, digits, dots, underscores or dashes, at most %d long", m.ID, idMaxLength)
	}
	return m.ID, nil
}

// copyTree copies from into dest, which it creates, under the same bounds as
// an archive. A link is refused rather than followed, so nothing outside the
// folder is copied.
func copyTree(from, dest string) error {
	budget, entries := int64(maxUnpackedBytes), 0
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entries++; entries > maxArchiveEntries {
			return fmt.Errorf("%s holds more than %d files", from, maxArchiveEntries)
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			budget, err = copyFile(path, rel, target, budget)
			return err
		default:
			return fmt.Errorf("%s is not a regular file or a folder; links are not accepted", rel)
		}
	})
}

func copyFile(path, rel, target string, budget int64) (int64, error) {
	src, err := os.Open(path)
	if err != nil {
		return budget, err
	}
	defer func() { _ = src.Close() }()
	return writeEntry(src, rel, target, budget)
}
