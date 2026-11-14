package claudeprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// The in-use markers Claude Code writes for each plugin version a session
// runs, one file per process named after its pid and holding {"pid":…}, and
// removed at its exit (read in 2.1.325): under plugins/.in_use-links/<version>
// when the version is a link, as a profile's are, and inside the version's
// .in_use/ when it is not, which is where an older Claude Code wrote through
// the link into the store's copy.
const (
	inUseLinksDir = ".in_use-links"
	inUseDir      = ".in_use"
)

// SweepPluginStore deletes each version in the store that no profile names,
// Personal included, and that no running Claude Code marks in use. It runs at
// lich start, before any session spawns: a session of an earlier run that ran
// a version its profile has since moved past ended with that run, and a Claude
// Code run outside lich with a profile's directory is kept off its versions by
// their markers. A record it cannot read stops the sweep, since the versions it
// names are unknown.
func SweepPluginStore() error {
	root, err := Root()
	if err != nil {
		return err
	}
	store := filepath.Join(root, pluginStoreName)
	pluginStoreMu.Lock()
	defer pluginStoreMu.Unlock()
	profiles, err := List()
	if err != nil {
		return err
	}
	named := map[string]bool{}
	for _, p := range profiles {
		versions, err := installedVersions(PluginsDir(p.Slug, p.Dir))
		if err != nil {
			return err
		}
		for _, version := range versions {
			named[version] = true
		}
	}
	stored, err := storedVersions(store)
	if err != nil {
		return err
	}
	for _, version := range stored {
		if named[version] || versionInUse(store, profiles, version) {
			continue
		}
		if err := dropVersion(store, profiles, version); err != nil {
			return err
		}
	}
	return nil
}

// storedVersions is every marketplace/plugin/version directory in the store.
// A link at any level is passed over, so nothing outside the store is reached.
func storedVersions(store string) ([]string, error) {
	paths := []string{""}
	for range versionDepth {
		var next []string
		for _, rel := range paths {
			names, err := realDirs(filepath.Join(store, rel))
			if err != nil {
				return nil, err
			}
			for _, name := range names {
				next = append(next, filepath.Join(rel, name))
			}
		}
		paths = next
	}
	return paths, nil
}

func realDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !profilestore.IsLink(filepath.Join(dir, e.Name())) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func versionInUse(store string, profiles []Profile, version string) bool {
	if markedInUse(filepath.Join(store, version, inUseDir)) {
		return true
	}
	for _, p := range profiles {
		if markedInUse(filepath.Join(PluginsDir(p.Slug, p.Dir), inUseLinksDir, version)) {
			return true
		}
	}
	return false
}

// markedInUse reports whether dir holds a marker of a running process. One it
// cannot read counts as running: Claude Code stages a marker under a temporary
// name before it holds the pid.
func markedInUse(dir string) bool {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	if err != nil {
		return true
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		var marker struct {
			PID int `json:"pid"`
		}
		if err != nil || json.Unmarshal(data, &marker) != nil || marker.PID <= 0 || processAlive(marker.PID) {
			return true
		}
	}
	return false
}

// dropVersion deletes a version from the store and each profile's link to it.
// os.RemoveAll removes a link inside the version without following it.
func dropVersion(store string, profiles []Profile, version string) error {
	if err := os.RemoveAll(filepath.Join(store, version)); err != nil {
		return fmt.Errorf("sweep plugin version %s: %w", version, err)
	}
	for _, p := range profiles {
		cache := filepath.Join(p.Dir, "plugins", "cache")
		link := filepath.Join(cache, version)
		if IsPersonal(p.Slug) || profilestore.IsLink(cache) || !profilestore.IsLink(link) {
			continue
		}
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("unlink %s: %w", link, err)
		}
	}
	return nil
}
