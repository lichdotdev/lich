package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
)

// The opencode side. Its sessions load the module from lich's own copy of the
// plugin through the inline config (bundle.go, internal/terminal), so the
// module a lich of the past wrote into opencode's plugin directory is removed:
// opencode loads that directory beside the inline config, and would run both.
// The theme skill is still written into opencode's skills directory
// (installOpencodeSkills), where opencode 1 needs a copy of its own.

const (
	// OpencodeModule is the module's path inside the plugin, and opencodeFile
	// the name a lich of the past gave it in opencode's plugin directory.
	OpencodeModule = "opencode/lich.js"
	opencodeFile   = "lich.js"

	// opencodeLegacyPluginDir is the directory lich wrote the module into
	// before opencode 2, which reads it no more.
	opencodeLegacyPluginDir = "plugin"

	// jsComment is how the marker line hides from the JavaScript that follows it.
	jsComment = "//"
)

func (s *Service) opencodeInstall() error {
	if err := s.installBundle(); err != nil {
		return err
	}
	version, _ := bundleVersion()
	dir, err := opencodePluginDir()
	if err != nil {
		return err
	}
	if err := s.installOpencodeSkills(version, filepath.Join(filepath.Dir(dir), "skills")); err != nil {
		return err
	}
	return removeOpencodeModules()
}

// removeOpencodeModules deletes the modules lich wrote into opencode's plugin
// directories, today's and the one before opencode 2.
func removeOpencodeModules() error {
	dir, err := opencodePluginDir()
	if err != nil {
		return err
	}
	if err := removeMarked(filepath.Join(dir, opencodeFile)); err != nil {
		return err
	}
	return removeLegacyOpencodeModule()
}

// removeMarked deletes a module lich wrote, which carries its marker line. A
// file there without it is somebody else's and stays.
func removeMarked(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if _, ours := versionOf(data, jsComment); !ours {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove the module lich installed at %s: %w", path, err)
	}
	return nil
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
	return removeMarked(path)
}

func legacyOpencodeModule() (string, error) {
	dir, err := opencodePluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(dir), opencodeLegacyPluginDir, opencodeFile), nil
}

// opencodeInstalledVersion is the version of the plugin opencode sessions load,
// once the install has copied the theme skill opencode reads from its own
// directory: until then the window keeps offering the install.
func opencodeInstalledVersion() (string, bool) {
	dir, err := opencodePluginDir()
	if err != nil || !skillPresent(filepath.Join(filepath.Dir(dir), "skills")) {
		return "", false
	}
	return bundleVersion()
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
