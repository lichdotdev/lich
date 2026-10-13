// Package ompprofile keeps more than one oh-my-pi login on one machine, the way
// internal/claudeprofile does for Claude Code. omp has profiles of its own: a
// name in OMP_PROFILE (or --profile) moves its whole state to
// ~/.omp/profiles/<name>, with its own agent.db holding the logins, its own
// sessions and settings, and nothing else in the session reads anything from
// there. A lich profile is one of those directories, which lich creates and
// links against Personal's.
//
// The default profile, Personal, is whatever omp reads without lich's help: the
// named profile in lich's own environment if there is one, else
// PI_CODING_AGENT_DIR, else ~/.omp/agent.
package ompprofile

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

// Profile is one oh-my-pi login lich knows about. Its Dir is the profile's agent
// directory (~/.omp/profiles/<slug>/agent): sessions, settings and agent.db.
type Profile = profilestore.Profile

// HomeVar is the variable that names omp's profile. Unlike the other providers'
// it holds the profile's name, not its directory: omp puts that under ~/.omp
// itself.
const HomeVar = "OMP_PROFILE"

// agentSubdir is where omp keeps what a profile is about, under the profile's
// directory.
const agentSubdir = "agent"

// store is what makes oh-my-pi's profiles oh-my-pi's. The shared entries are
// the user-level content omp reads from the agent directory (measured on
// 18.8.9: config.yml, models.yml, mcp.json, the markdown prompt files and the
// directories it discovers skills, rules, commands, prompts, extensions, hooks,
// tools, themes and modules from), linked so an edit made under one login is
// there under all of them. omp writes config.yml through a link and leaves the
// link in place (measured). Not shared: agent.db with the logins, sessions/,
// blobs/, its caches, and the plugins directory beside agent/, which is the
// profile's own set of `omp plugin install`.
var store = profilestore.Spec{
	Provider:    "omp",
	Label:       "oh-my-pi",
	HomeVar:     HomeVar,
	PersonalDir: PersonalDir,
	DirOf:       dirOf,
	SharedFiles: []string{
		"config.yml", "config.yaml", "models.yml", "mcp.json", ".mcp.json", "ssh.json",
		"AGENTS.md", "SYSTEM.md", "SYSTEM_TEMPLATE.md", "RULES.md", "PERSONALITY.md",
	},
	Shared: []string{
		"agents", "commands", "extensions", "hooks", "modules", "prompts", "rules", "skills", "themes", "tools",
	},
	// A broker named in the environment is read before the profile's own
	// agent.db, so every profile would spend the broker's account.
	Override: []string{"OMP_AUTH_BROKER_URL", "OMP_AUTH_BROKER_TOKEN"},
	// "default" is omp's name for the default profile: OMP_PROFILE=default
	// selects ~/.omp/agent, which would put a login meant for a profile there.
	Reserved: []string{"default"},
}

// Root is where lich keeps its oh-my-pi profile sidecars. The profiles'
// directories are omp's, under ~/.omp/profiles.
func Root() (string, error) { return store.Root() }

// Dir is the agent directory of a profile lich created. It does not check that
// the profile exists; ConfigDir does.
func Dir(slug string) (string, error) { return store.Dir(slug) }

func dirOf(slug string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".omp", "profiles", slug, agentSubdir), nil
}

// PersonalDir is the default profile's agent directory: what omp resolves from
// lich's own environment. A named profile wins outright over the explicit
// override, then the default, in the precedence `omp config path` was measured
// to apply (18.8.9).
func PersonalDir() (string, error) {
	if profile := os.Getenv(HomeVar); profile != "" {
		return dirOf(profile)
	}
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".omp", agentSubdir), nil
}

// ConfigDir is a profile's agent directory, Personal's included. A profile that
// does not exist answers ErrProfileGone.
func ConfigDir(slug string) (string, error) { return store.ConfigDir(slug) }

// IsPersonal reports whether slug names the default profile. Empty does too: it
// is what every session row born before profiles carries.
func IsPersonal(slug string) bool { return profilestore.IsPersonal(slug) }

// List is every profile, Personal first and the rest in the order they were
// created.
func List() ([]Profile, error) { return store.List() }

// ConfigDirs is every profile, Personal first. Every reader of omp's sessions
// scans all of them: a conversation lives in exactly one, and which one is what
// says the profile it belongs to.
func ConfigDirs() []Profile { return store.All() }

// Create makes a profile named name: its agent directory, laid out against
// Personal's, and its sidecar. It does not log in.
func Create(name string) (Profile, error) { return store.Create(name) }

// Rename changes a profile's display name. The slug and the directory stay.
func Rename(slug, name string) error { return store.Rename(slug, name) }

// Delete removes a profile with its login, its sessions and everything omp
// kept under ~/.omp/profiles/<slug>. Personal cannot be deleted.
func Delete(slug string) error {
	if IsPersonal(slug) {
		return store.Delete(slug)
	}
	dir, err := store.ConfigDir(slug)
	if err != nil {
		return err
	}
	if err := store.Delete(slug); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(dir)); err != nil {
		return fmt.Errorf("delete %s: %w", filepath.Dir(dir), err)
	}
	return nil
}

// EnsureShared links into a profile the shared entries Personal gained since
// the profile was made, which runs before every spawn into it.
func EnsureShared(slug string) error { return store.EnsureShared(slug) }

// Env is base with OMP_PROFILE naming the profile whose agent directory is dir
// and the variables that would override its login removed.
func Env(base []string, dir string) []string {
	return store.Env(base, slugOf(dir))
}

// slugOf is the name of the profile whose agent directory is dir.
func slugOf(dir string) string { return filepath.Base(filepath.Dir(dir)) }
