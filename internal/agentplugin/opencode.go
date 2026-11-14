package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
)

// The opencode side. opencode has no plugin CLI and no marketplace: a plugin
// there is a JavaScript module its server imports from a directory, so
// installing one is writing the file and uninstalling one is deleting it.
//
// lich writes the module the release published, with a marker line naming the
// version above it — that line is the only record of what is installed, since
// opencode keeps no plugin state to ask.

const (
	// opencodeSource is the module's path inside the plugin repository, and
	// opencodeFile the name it takes in opencode's plugin directory. The
	// extension is load-bearing: opencode 1.x globs `{plugin,plugins}/*.{ts,js}`
	// and 2.x `plugins/*.{ts,js}`, so a `.mjs` would sit there unread.
	opencodeSource = "opencode/lich.js"
	opencodeFile   = "lich.js"

	// opencodeLegacyPluginDir is the directory lich wrote the module into
	// before opencode 2, which reads it no more.
	opencodeLegacyPluginDir = "plugin"

	// jsComment is how the marker line hides from the JavaScript that follows it.
	jsComment = "//"
)

func (s *Service) opencodeInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	data, err := s.fetchFile(version, opencodeSource)
	if err != nil {
		return err
	}
	dir, err := opencodePluginDir()
	if err != nil {
		return err
	}
	if err := s.installOpencodeSkills(version, filepath.Join(filepath.Dir(dir), "skills")); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	body := fmt.Sprintf("%s %s v%s — installed by lich, replace with the file from %s\n%s",
		jsComment, markerName, version, marketplaceRepo, data)
	if err := writeFile(filepath.Join(dir, opencodeFile), []byte(body), 0o644); err != nil {
		return err
	}
	return removeLegacyOpencodeModule()
}

// removeLegacyOpencodeModule deletes the module an older lich wrote into
// `plugin/`, which only opencode 1.x reads: 1.x reads `plugins/` too, and would
// load both copies and report every event twice. A module there without lich's
// marker is somebody else's and stays.
func removeLegacyOpencodeModule() error {
	path, err := legacyOpencodeModule()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if _, ours := versionOf(data, jsComment); !ours {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove the module an older lich installed at %s: %w", path, err)
	}
	return nil
}

func legacyOpencodeModule() (string, error) {
	dir, err := opencodePluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(dir), opencodeLegacyPluginDir, opencodeFile), nil
}

// opencodeInstalledVersion reads the version off the marker line lich wrote, or
// ("", false) when the module is absent, unreadable, or was not written by lich
// — a hand-installed copy carries no marker, and reporting a version for it
// would claim an install lich cannot update.
//
// The module an older lich left in `plugin/` still counts, so its user is
// offered the update that moves it.
func (s *Service) opencodeInstalledVersion() (string, bool) {
	dir, err := opencodePluginDir()
	if err != nil {
		return "", false
	}
	legacy, err := legacyOpencodeModule()
	if err != nil {
		return "", false
	}
	for _, path := range []string{filepath.Join(dir, opencodeFile), legacy} {
		if data, err := os.ReadFile(path); err == nil {
			if version, ok := versionOf(data, jsComment); ok {
				return version, true
			}
		}
	}
	return "", false
}

// opencodePluginDir resolves opencode's global plugin directory, the one both
// generations read (opencode2's docs, `~/.config/opencode/plugins/`). opencode reads
// it through the xdg-basedir convention — $XDG_CONFIG_HOME, else ~/.config —
// which it applies on every platform rather than deferring to the OS-native
// config location, so os.UserConfigDir would point elsewhere on macOS and
// Windows and the module would land where nothing loads it.
func opencodePluginDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "opencode", "plugins"), nil
}
