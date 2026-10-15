package profilestore

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// linked is every entry a profile holds as a link to Personal's, files first.
func (s Spec) linked() []string {
	return slices.Concat(s.SharedFiles, s.Shared, s.Downloads)
}

func (s Spec) personalPath(personal, name string) string {
	if s.PersonalPath == nil {
		return filepath.Join(personal, name)
	}
	return s.PersonalPath(personal, name)
}

// build lays a new profile out at dir against Personal. It reports the shared
// entries it had to copy instead of link.
func (s Spec) build(dir string) ([]string, error) {
	personal, err := s.PersonalDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(dir), err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	for _, name := range s.Downloads {
		if err := os.MkdirAll(s.personalPath(personal, name), 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", s.personalPath(personal, name), err)
		}
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700); err != nil {
			return nil, err
		}
	}
	copied, err := s.linkShared(personal, dir)
	if err != nil {
		return nil, err
	}
	if s.Seed != nil {
		if err := s.Seed(personal, dir); err != nil {
			return nil, err
		}
	}
	if s.Sync != nil {
		if err := s.Sync(personal, dir); err != nil {
			return nil, err
		}
	}
	return copied, nil
}

// linkShared links every shared entry Personal has and the profile lacks. An
// entry the profile already holds is left alone, link or not: the provider may
// have written a settings file of its own there before Personal had one, and
// that file is the profile's now.
func (s Spec) linkShared(personal, dir string) ([]string, error) {
	if len(s.Own) > 0 {
		if err := s.tidyHome(dir, ""); err != nil {
			return nil, err
		}
		return nil, s.linkHome(personal, dir, "")
	}
	var copied []string
	for _, name := range s.linked() {
		target := s.personalPath(personal, name)
		path := filepath.Join(dir, name)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			continue
		}
		if !slices.Contains(s.SharedFiles, name) {
			if err := LinkDir(target, path); err != nil {
				return copied, fmt.Errorf("link %s: %w", path, err)
			}
			continue
		}
		wasCopied, err := linkFile(target, path)
		if err != nil {
			return copied, fmt.Errorf("link %s: %w", path, err)
		}
		if wasCopied {
			copied = append(copied, name)
		}
	}
	return copied, nil
}

// EnsureShared links into a profile the shared entries Personal gained since
// the profile was made (a CLAUDE.md written after it, say), records any it had
// to copy and runs Sync. It runs before every spawn into the profile, so an
// entry appears on the next session. Personal needs nothing.
func (s Spec) EnsureShared(slug string) error {
	if IsPersonal(slug) {
		return nil
	}
	dir, err := s.ConfigDir(slug)
	if err != nil {
		return err
	}
	unlock := lockProfile(dir)
	defer unlock()
	personal, err := s.PersonalDir()
	if err != nil {
		return err
	}
	copied, err := s.linkShared(personal, dir)
	if err != nil {
		return err
	}
	if s.Sync != nil {
		if err := s.Sync(personal, dir); err != nil {
			return err
		}
	}
	if len(copied) == 0 {
		return nil
	}
	m, err := s.readMeta(slug)
	if err != nil {
		return err
	}
	m.Copied = append(m.Copied, copied...)
	return s.writeMeta(slug, m)
}

// profileLocks holds a mutex per profile directory: two spawns into one
// profile run EnsureShared at once, and its read-modify-write of the meta file
// would lose one of them's copies.
var profileLocks sync.Map

func lockProfile(dir string) (unlock func()) {
	m, _ := profileLocks.LoadOrStore(filepath.Clean(dir), new(sync.Mutex))
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
