// Package claudeprofile keeps more than one Claude Code account on one machine.
// A profile is a config directory of its own (CLAUDE_CONFIG_DIR) holding its own
// `claude auth login`, so every account runs with its full login rather than an
// inference-only token. The default profile, Personal, is whatever Claude Code
// reads without lich's help: CLAUDE_CONFIG_DIR from lich's environment, else
// ~/.claude.
//
// A profile directory is never moved once it exists. On macOS Claude Code files
// the login in the Keychain under a service named after a hash of the directory
// string (internal/quota/claude_storage.go), so Dir must return the same bytes
// for a slug on every call, and renaming a profile changes its display name only.
package claudeprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/omartelo/lich/internal/profilestore"
)

// Personal is the slug of the default profile. It is reserved: no profile lich
// creates can take it, which is what lets a project setting name Personal
// explicitly where an empty value means "inherit the global one".
const Personal = profilestore.Personal

// ErrProfileGone is the error a spawn returns for a session born in a profile
// that was deleted since. The window matches it to offer a new session in the
// project's own profile instead.
var ErrProfileGone = profilestore.ErrGone

// Profile is one Claude Code account lich knows about.
type Profile = profilestore.Profile

// store is what makes Claude Code's profiles Claude Code's: where the default
// profile is, what a profile links from it, and the variables that would
// override a profile's own login.
var store = profilestore.Spec{
	Provider:    "claude",
	Label:       "Claude Code",
	HomeVar:     "CLAUDE_CONFIG_DIR",
	PersonalDir: PersonalDir,
	// Claude Code writes through a link to its target (measured on 2.1.296).
	SharedFiles: []string{"settings.json", "CLAUDE.md"},
	Shared:      []string{"agents", "commands", "skills", "hooks"},
	// Personal's marketplace clones, so a profile does not clone every
	// marketplace again. The two state files beside them stay per profile: an
	// organization account may forbid plugins a personal one runs, so the
	// installed set differs per account. The plugin cache is the profile's own
	// (plugincache.go).
	Downloads: []string{filepath.Join("plugins", "marketplaces")},
	// The secure-storage override goes too, since on macOS it would point every
	// profile at one Keychain item, and so does the plugins override, which would
	// point every profile at one installed set (PluginsDir).
	Override: []string{
		"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_SECURESTORAGE_CONFIG_DIR",
		pluginsDirVar,
	},
	Seed: seed,
	Sync: layoutPluginCache,
}

// Root is where lich keeps its Claude profiles.
func Root() (string, error) { return store.Root() }

// Dir is the CLAUDE_CONFIG_DIR of a profile lich created. It does not check
// that the profile exists; ConfigDir does.
func Dir(slug string) (string, error) { return store.Dir(slug) }

// PersonalDir is the default profile's config directory: what Claude Code
// resolves from lich's own environment, CLAUDE_CONFIG_DIR else ~/.claude.
func PersonalDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// pluginsDirVar moves Claude Code's whole plugins directory, not only the cache
// its name says: installed_plugins.json, the marketplaces and the cache are
// read under it (measured on 2.1.300). A leading ~ is the home directory.
const pluginsDirVar = "CLAUDE_CODE_PLUGIN_CACHE_DIR"

// PluginsDir is the plugins directory of the profile slug whose config
// directory is dir. Personal's follows pluginsDirVar from lich's environment; a
// profile's is always inside its own, since its spawn drops the variable.
func PluginsDir(slug, dir string) string {
	if IsPersonal(slug) {
		return personalPluginsDir(dir)
	}
	return filepath.Join(dir, "plugins")
}

func personalPluginsDir(personal string) string {
	dir := os.Getenv(pluginsDirVar)
	if dir == "" {
		return filepath.Join(personal, "plugins")
	}
	if dir != "~" && !strings.HasPrefix(dir, "~/") {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return dir
	}
	return filepath.Join(home, dir[1:])
}

// personalGlobalFile is the default profile's global config: inside the config
// dir when CLAUDE_CONFIG_DIR names one, else ~/.claude.json beside ~/.claude.
func personalGlobalFile() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); filepath.IsAbs(dir) {
		return filepath.Join(dir, ".claude.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

// ConfigDir is a profile's config directory, Personal's included. A profile
// that does not exist answers ErrProfileGone.
func ConfigDir(slug string) (string, error) { return store.ConfigDir(slug) }

// IsPersonal reports whether slug names the default profile. Empty does too: it
// is what every session row born before profiles carries.
func IsPersonal(slug string) bool { return profilestore.IsPersonal(slug) }

// List is every profile, Personal first and the rest in the order they were
// created.
func List() ([]Profile, error) { return store.List() }

// ConfigDirs is every profile's config directory keyed by slug, Personal first.
// Every reader of Claude Code's transcripts scans all of them: a conversation
// lives in exactly one, and which one is what says the profile it belongs to.
func ConfigDirs() []Profile { return store.All() }

// Create makes a profile named name: its directory, laid out against Personal
// (layout.go), and its sidecar. It does not log in.
func Create(name string) (Profile, error) { return store.Create(name) }

// Rename changes a profile's display name. The slug and the directory stay.
func Rename(slug, name string) error { return store.Rename(slug, name) }

// Delete removes a profile's directory and its login with it. Personal cannot
// be deleted.
func Delete(slug string) error {
	if IsPersonal(slug) {
		return store.Delete(slug)
	}
	dir, err := ConfigDir(slug)
	if err != nil {
		return err
	}
	if err := unlinkPluginVersions(dir); err != nil {
		return err
	}
	return store.Delete(slug)
}

// EnsureShared links into a profile the shared entries Personal gained since
// the profile was made, which runs before every spawn into it.
func EnsureShared(slug string) error { return store.EnsureShared(slug) }

// Env is base with a profile's directory as CLAUDE_CONFIG_DIR and the variables
// that would override its login removed.
func Env(base []string, dir string) []string { return store.Env(base, dir) }
