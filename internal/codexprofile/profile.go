// Package codexprofile keeps more than one Codex login on one machine, the way
// internal/claudeprofile does for Claude Code. A profile is a state directory of
// its own (CODEX_HOME) holding its own `codex login`, so every account runs with
// its full login. The default profile, Personal, is whatever Codex reads without
// lich's help: CODEX_HOME from lich's environment, else ~/.codex.
//
// A profile directory is never moved once it exists, and renaming a profile
// changes its display name only: the slug names the directory, and Codex may key
// a login on the directory string (its keyring store does).
package codexprofile

import (
	"fmt"
	"os"
	"path/filepath"

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

// Profile is one Codex login lich knows about.
type Profile = profilestore.Profile

// HomeVar is the variable Codex reads its state directory from.
const HomeVar = "CODEX_HOME"

// store is what makes Codex's profiles Codex's: where the default profile is,
// what a profile links from it, and the variables that would override a
// profile's own state or login.
var store = profilestore.Spec{
	Provider:    "codex",
	Label:       "Codex",
	HomeVar:     HomeVar,
	PersonalDir: PersonalDir,
	// config.toml is not linked: it holds the plugin set too, which is per
	// profile, so a profile keeps a file of its own that Sync rebuilds from
	// Personal's before every spawn (config.go). The plugin and marketplace
	// downloads are the profile's own for the same reason.
	SharedFiles: []string{"hooks.json", "AGENTS.md"},
	Shared:      []string{"skills", "rules"},
	// CODEX_SQLITE_HOME moves Codex's thread database out of the home, which
	// would put every profile's threads in one place.
	Override: []string{"CODEX_API_KEY", "CODEX_ACCESS_TOKEN", "OPENAI_API_KEY", "CODEX_SQLITE_HOME"},
	Sync:     syncConfig,
}

// Root is where lich keeps its Codex profiles, beside the Claude Code ones.
func Root() (string, error) { return store.Root() }

// Dir is the CODEX_HOME of a profile lich created. It does not check that the
// profile exists; ConfigDir does.
func Dir(slug string) (string, error) { return store.Dir(slug) }

// PersonalDir is the default profile's state directory: what Codex resolves from
// lich's own environment.
func PersonalDir() (string, error) {
	if dir := os.Getenv(HomeVar); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex"), nil
}

// ConfigDir is a profile's state directory, Personal's included. A profile that
// does not exist answers ErrProfileGone.
func ConfigDir(slug string) (string, error) { return store.ConfigDir(slug) }

// IsPersonal reports whether slug names the default profile. Empty does too: it
// is what every session row born before profiles carries.
func IsPersonal(slug string) bool { return profilestore.IsPersonal(slug) }

// List is every profile, Personal first and the rest in the order they were
// created.
func List() ([]Profile, error) { return store.List() }

// ConfigDirs is every profile, Personal first. Every reader of Codex's rollouts
// scans all of them: a conversation lives in exactly one, and which one is what
// says the profile it belongs to.
func ConfigDirs() []Profile { return store.All() }

// Create makes a profile named name: its directory, laid out against Personal
// (layout.go), and its sidecar. It does not log in.
func Create(name string) (Profile, error) { return store.Create(name) }

// Rename changes a profile's display name. The slug and the directory stay.
func Rename(slug, name string) error { return store.Rename(slug, name) }

// Delete removes a profile's directory and its login with it. Personal cannot
// be deleted.
func Delete(slug string) error { return store.Delete(slug) }

// EnsureShared links into a profile the shared entries Personal gained since
// the profile was made and rebuilds its config.toml from Personal's, which runs
// before every spawn into it.
func EnsureShared(slug string) error { return store.EnsureShared(slug) }

// Env is base with a profile's directory as CODEX_HOME and the variables that
// would override its login removed.
func Env(base []string, dir string) []string { return store.Env(base, dir) }
