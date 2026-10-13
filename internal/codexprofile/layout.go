package codexprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// sharedEntries are the user's own Codex content, linked from every profile to
// Personal so an edit made under one account is there under all of them. Codex
// writes through a link to its target and leaves the link in place (measured on
// 0.162.1 with `features enable`). Only entries Personal has are linked;
// EnsureShared links one created later.
//
// config.toml also holds the installed plugin set, the marketplaces and the
// trusted hooks, so sharing it shares those too (sharedDownloads is what lets
// them resolve).
var sharedEntries = []string{"config.toml", "hooks.json", "AGENTS.md", "skills", "rules"}

// sharedFiles are the entries that are files rather than directories, the ones
// Windows can only link with Developer Mode and otherwise copies.
var sharedFiles = map[string]bool{"config.toml": true, "hooks.json": true, "AGENTS.md": true}

// sharedDownloads are the plugin and marketplace downloads that the plugin
// entries of the shared config.toml point at, linked so a plugin installed in
// one profile is installed in all of them. `plugin marketplace add` and
// `plugin add` run in a profile land in Personal's through these links
// (measured on 0.162.1).
var sharedDownloads = []string{
	filepath.Join("plugins", "cache"),
	filepath.Join(".tmp", "marketplaces"),
}

// build lays a new profile out at dir against Personal. It reports the shared
// entries it had to copy instead of link.
func build(dir string) ([]string, error) {
	personal, err := PersonalDir()
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	for _, name := range sharedDownloads {
		if err := os.MkdirAll(filepath.Join(personal, name), 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", filepath.Join(personal, name), err)
		}
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700); err != nil {
			return nil, err
		}
	}
	return linkShared(personal, dir)
}

// linkShared links every shared entry Personal has and the profile lacks. An
// entry the profile already holds is left alone, link or not: Codex may have
// written a config.toml of its own there before Personal had one, and that file
// is the profile's now.
func linkShared(personal, dir string) ([]string, error) {
	var copied []string
	for _, name := range append(slices.Clone(sharedEntries), sharedDownloads...) {
		target := filepath.Join(personal, name)
		path := filepath.Join(dir, name)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			continue
		}
		if !sharedFiles[name] {
			if err := linkDir(target, path); err != nil {
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

// EnsureShared links into a profile the shared entries Personal gained since the
// profile was made (an AGENTS.md written after it, say) and records any it had
// to copy. It runs before every spawn into the profile, so an entry appears on
// the next session. Personal needs nothing.
func EnsureShared(slug string) error {
	if IsPersonal(slug) {
		return nil
	}
	dir, err := ConfigDir(slug)
	if err != nil {
		return err
	}
	personal, err := PersonalDir()
	if err != nil {
		return err
	}
	copied, err := linkShared(personal, dir)
	if err != nil || len(copied) == 0 {
		return err
	}
	m, err := readMeta(dir)
	if err != nil {
		return err
	}
	m.Copied = append(m.Copied, copied...)
	return writeMeta(dir, m)
}
