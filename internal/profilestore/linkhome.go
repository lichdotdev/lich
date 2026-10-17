package profilestore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// A link home is the profile directory of a provider that keeps its login under
// an XDG base directory every other tool reads too (internal/xdgprofile): the
// session's XDG_*_HOME points at it, and it mirrors Personal's base directory
// entry by entry, each a link, so gh, git and mise find their own state as if
// nothing had moved. What Spec.Own names is the profile's instead, and every
// directory on the way to one is a real directory mirrored the same way. The
// entry holding the profile itself (lich's config dir, when the base directory
// is XDG_CONFIG_HOME) is never linked: it would be a cycle.

// ownership is whether rel, relative to the home, is one of Own's entries or a
// directory holding one.
func (s Spec) ownership(rel string) (own, holds bool) {
	for _, o := range s.Own {
		if o == rel {
			return true, false
		}
		if strings.HasPrefix(o, rel+string(filepath.Separator)) {
			holds = true
		}
	}
	return false, holds
}

// linkHome lays rel of Personal's home out in dir: an entry the profile lacks
// is linked, a directory holding one of Own's entries is made and mirrored, and
// what the profile already holds is left alone, as linkShared does. The entry
// holding dir itself is never linked.
func (s Spec) linkHome(personal, dir, rel string) error {
	entries, err := os.ReadDir(filepath.Join(personal, rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := filepath.Join(rel, e.Name())
		own, holds := s.ownership(name)
		if own {
			continue
		}
		path := filepath.Join(dir, name)
		if holds {
			if err := s.mirrorDir(personal, dir, name); err != nil {
				return err
			}
			continue
		}
		target := filepath.Join(personal, name)
		if _, err := os.Lstat(path); err == nil || holdsDir(target, dir) {
			continue
		}
		if err := linkEntry(target, path); err != nil {
			return fmt.Errorf("link %s: %w", path, err)
		}
	}
	return nil
}

// linkEntry links path to Personal's entry target: a directory with LinkDir, a
// file with linkHomeFile, since a Windows junction stands only for a directory.
func linkEntry(target, path string) error {
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return LinkDir(target, path)
	}
	return linkHomeFile(target, path)
}

// mirrorDir makes the directory rel holding one of Own's entries and lays it
// out. One that is a link is Personal's own directory: walking it would lay
// links out inside Personal's.
func (s Spec) mirrorDir(personal, dir, rel string) error {
	path := filepath.Join(dir, rel)
	if IsLink(path) {
		return nil
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return s.linkHome(personal, dir, rel)
}

// tidyHome keeps a link home in step with Personal before a spawn: it never
// writes into Personal's directory. A link whose target Personal no longer
// has is removed, so a tool can make the entry again, and so is one that
// reaches the profile itself. An entry a tool made under the profile is the
// profile's own from then on, whether Personal has one or not.
func (s Spec) tidyHome(dir, rel string) error {
	entries, err := os.ReadDir(filepath.Join(dir, rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := filepath.Join(rel, e.Name())
		path := filepath.Join(dir, name)
		own, holds := s.ownership(name)
		switch {
		case own:
			continue
		case IsLink(path):
			if err := pruneLink(path, dir); err != nil {
				return err
			}
		case holds:
			if err := s.tidyHome(dir, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// pruneLink removes the link path when its target is gone or holds the link
// home dir.
func pruneLink(path, dir string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return fmt.Errorf("read link %s: %w", path, err)
	}
	_, statErr := os.Stat(path)
	if !errors.Is(statErr, fs.ErrNotExist) && !holdsDir(target, dir) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("unlink %s: %w", path, err)
	}
	return nil
}

// holdsDir is whether target is dir or a directory above it: a link to it from
// inside dir would be a cycle. For a provider whose base directory is the one
// lich's own config lives in (Cursor CLI's XDG_CONFIG_HOME), that is the entry
// holding the profiles.
func holdsDir(target, dir string) bool {
	target, dir = filepath.Clean(target), filepath.Clean(dir)
	return target == dir || strings.HasPrefix(dir, target+string(filepath.Separator))
}
