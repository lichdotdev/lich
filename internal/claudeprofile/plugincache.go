package claudeprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/omartelo/lich/internal/profilestore"
)

// A profile keeps a plugins/cache of its own, and every version its
// installed_plugins.json names is a link into one store of versions shared by
// all profiles, so a version is downloaded and kept once. A link and not the
// directory because Claude Code's orphan sweep, run at every session start,
// marks each cached version its own record does not name and deletes it 14
// days later, but passes over a version that is a link, marking and deleting
// nothing through it (measured on 2.1.299). With one cache shared by every
// profile, a plugin only one of them had was deleted under it.
//
// Personal's cache is left alone: its versions are Personal's own, swept by
// Personal's sessions against Personal's record.

// pluginStoreName is the store under lich's Claude profiles root. It is not a
// slug, so no profile can take it.
const pluginStoreName = "plugin_cache"

// replacedSuffix names a profile's own copy of a version the store already
// holds while it is swapped for a link. One that could not be removed is a
// version the profile does not name, so Claude Code's sweep deletes it.
const replacedSuffix = ".lich-replaced"

// pluginStoreMu serializes the store's writes: two profiles spawning at once
// may adopt the same version.
var pluginStoreMu sync.Mutex

// PluginStore is the store of the profile at dir, which sits under lich's
// Claude profiles root like every profile lich creates.
func PluginStore(dir string) string {
	return filepath.Join(filepath.Dir(dir), pluginStoreName)
}

// layoutPluginCache is the Sync of a Claude profile: it gives the profile a
// cache of its own, moving one that still links to Personal's, and turns each
// version the profile names into a link into the store. A version it does not
// name stays as Claude Code left it, for its sweep to decide.
func layoutPluginCache(_, dir string) error {
	store := PluginStore(dir)
	cache := filepath.Join(dir, "plugins", "cache")
	versions, err := installedVersions(dir, cache)
	if err != nil {
		return err
	}
	pluginStoreMu.Lock()
	defer pluginStoreMu.Unlock()
	if profilestore.IsLink(cache) {
		if err := unlinkSharedCache(cache, store, versions); err != nil {
			return err
		}
	} else if err := os.MkdirAll(cache, 0o700); err != nil {
		return err
	}
	for _, version := range versions {
		if err := linkVersion(filepath.Join(cache, version), filepath.Join(store, version)); err != nil {
			return err
		}
	}
	return nil
}

// installedVersions is every marketplace/plugin/version under cache the
// profile's installed_plugins.json names.
func installedVersions(dir, cache string) ([]string, error) {
	path := filepath.Join(dir, "plugins", "installed_plugins.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc struct {
		Plugins map[string][]struct {
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var versions []string
	for _, installs := range doc.Plugins {
		for _, install := range installs {
			rel, err := filepath.Rel(cache, filepath.FromSlash(install.InstallPath))
			if err != nil || !filepath.IsLocal(rel) || len(strings.Split(rel, string(filepath.Separator))) != 3 {
				continue
			}
			versions = append(versions, rel)
		}
	}
	slices.Sort(versions)
	return slices.Compact(versions), nil
}

// unlinkSharedCache turns a cache that links to Personal's, from a profile
// made when the whole cache was shared, into a directory of the profile's own,
// first copying into the store each version the profile names. A copy, since
// Personal may run the same version. One already gone from Personal's cache
// is left to Claude Code, which finds its record naming nothing.
func unlinkSharedCache(cache, store string, versions []string) error {
	for _, version := range versions {
		shared, src := filepath.Join(store, version), filepath.Join(cache, version)
		stored, err := pathExists(shared)
		if err != nil {
			return err
		}
		inPersonal, err := pathExists(src)
		if err != nil {
			return err
		}
		if stored || !inPersonal {
			continue
		}
		if err := copyVersion(src, shared); err != nil {
			return fmt.Errorf("copy plugin version %s: %w", src, err)
		}
	}
	if err := os.Remove(cache); err != nil {
		return fmt.Errorf("unlink %s: %w", cache, err)
	}
	return os.Mkdir(cache, 0o700)
}

// linkVersion makes path, a version the profile names, a link to shared in
// the store: a version Claude Code downloaded there moves into the store, or
// gives way to the store's copy. One the store lacks and the profile lacks
// too stays missing.
func linkVersion(path, shared string) error {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		stored, err := pathExists(shared)
		if err != nil || !stored {
			return err
		}
	case err != nil:
		return err
	case profilestore.IsLink(path) || !info.IsDir():
		return nil
	default:
		adopted, err := adoptVersion(path, shared)
		if err != nil || !adopted {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := profilestore.LinkDir(shared, path); err != nil {
		return fmt.Errorf("link %s: %w", path, err)
	}
	return nil
}

// adoptVersion moves the profile's own version at path out of the way of a
// link: into the store, or aside when the store already holds it. A version
// that cannot be moved, which Windows refuses while a file in it is open,
// stays the profile's own until the next spawn: it is named by the profile's
// record, so no sweep touches it.
func adoptVersion(path, shared string) (bool, error) {
	exists, err := pathExists(shared)
	if err != nil {
		return false, err
	}
	if !exists {
		if err := os.MkdirAll(filepath.Dir(shared), 0o700); err != nil {
			return false, err
		}
		if err := os.Rename(path, shared); err != nil {
			slog.Warn("claudeprofile: plugin version stays the profile's own", "path", path, "err", err)
			return false, nil
		}
		return true, nil
	}
	replaced := path + replacedSuffix
	if err := os.RemoveAll(replaced); err != nil {
		return false, err
	}
	if err := os.Rename(path, replaced); err != nil {
		slog.Warn("claudeprofile: plugin version stays the profile's own", "path", path, "err", err)
		return false, nil
	}
	if err := os.RemoveAll(replaced); err != nil {
		slog.Warn("claudeprofile: replaced plugin version left for Claude Code's sweep", "path", replaced, "err", err)
	}
	return true, nil
}

// copyVersion copies the version directory src to dst through a staging
// directory beside dst, so the store never holds half a version. It keeps the
// permission bits, a plugin's scripts being executables, and copies links as
// links.
func copyVersion(src, dst string) error {
	staging := dst + ".partial"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		to := filepath.Join(staging, rel)
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, to)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(to, info.Mode().Perm()|0o700)
		}
		return copyFileMode(path, to, info.Mode().Perm())
	})
	if err == nil {
		err = os.Rename(staging, dst)
	}
	if err != nil {
		_ = os.RemoveAll(staging)
	}
	return err
}

func copyFileMode(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// unlinkPluginVersions removes a profile's links into the store, or its link
// to Personal's whole cache, so deleting the profile never walks through one.
func unlinkPluginVersions(dir string) error {
	cache := filepath.Join(dir, "plugins", "cache")
	links := []string{cache}
	if !profilestore.IsLink(cache) {
		// Globbed inside cache, which may hold metacharacters of its own.
		versions, err := fs.Glob(os.DirFS(cache), "*/*/*")
		if err != nil {
			return err
		}
		links = links[:0]
		for _, version := range versions {
			links = append(links, filepath.Join(cache, filepath.FromSlash(version)))
		}
	}
	for _, path := range links {
		if !profilestore.IsLink(path) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("unlink %s: %w", path, err)
		}
	}
	return nil
}

func pathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
