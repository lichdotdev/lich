// Package xdgprofile keeps more than one login of opencode, Cursor CLI and Kiro
// CLI on one machine, the way internal/claudeprofile does for Claude Code. None
// of the three has a variable of its own that moves its login: opencode keeps it
// in $XDG_DATA_HOME/opencode, Cursor CLI in $XDG_CONFIG_HOME/cursor (on Windows
// %APPDATA%\Cursor) and Kiro CLI in $XDG_DATA_HOME/kiro-cli/data.sqlite3, all
// measured (docs/providers/, Profiles). A profile is therefore a base directory of its
// own that the session's XDG_DATA_HOME, XDG_CONFIG_HOME or APPDATA points at,
// made of links to every entry of the real one but the provider's
// (profilestore's link home), so every other tool in the session (gh, git,
// mise) reads its own state where it always was.
//
// Which of the three have profiles depends on the platform (Providers): on
// macOS Cursor CLI and Kiro CLI keep their logins in the Keychain, and on
// Windows Kiro CLI finds its data folder with no variable in the way.
package xdgprofile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
)

// Personal is the slug of the default profile.
const Personal = profilestore.Personal

// ErrProfileGone is the error a spawn returns for a session born in a profile
// that was deleted since.
var ErrProfileGone = profilestore.ErrGone

// Profile is one login lich knows about. Its Dir is the profile's base
// directory, the link home; Personal's is the real one.
type Profile = profilestore.Profile

const (
	configHomeVar      = "XDG_CONFIG_HOME"
	dataHomeVar        = "XDG_DATA_HOME"
	cursorConfigDirVar = "CURSOR_CONFIG_DIR"
)

// Provider is one provider whose profiles are link homes.
type Provider struct {
	// ID is the provider's id (providers.OpenCode).
	ID    string
	store profilestore.Spec
	// folder is the provider's own directory under the base directory.
	folder string
	// personalState is Personal's state directory, which for Cursor CLI is not
	// always under the base directory (providers.CursorConfigDir).
	personalState func() (string, error)
	// status is the CLI call that says who is logged in, and login the
	// arguments that log a profile in from a terminal.
	status statusCommand
	login  string
}

// OpenCode is opencode's profiles. Its own folder, `opencode/` under the data
// home, holds the login (the `credential` and `account` tables of opencode.db,
// beside auth.json) together with every conversation, so the whole folder is
// the profile's, and a conversation cannot move between profiles. The config
// (`~/.config/opencode`, where lich's plugin is) is not under the data home and
// is every profile's. Dropped from a profile's spawn: the variables that hand
// opencode a database or a login of their own.
var OpenCode = &Provider{
	ID: providers.OpenCode,
	store: profilestore.Spec{
		Provider:    providers.OpenCode,
		Label:       "opencode",
		HomeVar:     dataHomeVar,
		PersonalDir: dataHome,
		Own:         []string{"opencode"},
		Override:    []string{"OPENCODE_DB", "OPENCODE_AUTH_CONTENT", "OPENCODE_CONSOLE_TOKEN", "OPENCODE_API_KEY"},
	},
	folder:        "opencode",
	personalState: func() (string, error) { return under(dataHome, "opencode") },
	status:        opencodeStatus,
	login:         "auth login",
}

// Cursor is Cursor CLI's profiles. With XDG_CONFIG_HOME set and no
// CURSOR_CONFIG_DIR, the CLI keeps its login (auth.json), its config
// (cli-config.json, which names the account) and its chats in
// $XDG_CONFIG_HOME/cursor, so that folder is the profile's. On Windows the login
// is %APPDATA%\Cursor\auth.json and the config dir ~/.cursor, so there a
// profile moves APPDATA and pins CURSOR_CONFIG_DIR to that same folder
// (cursor_windows.go). Its mcp.json, hooks and skills hang off ~/.cursor
// whatever is set, and are every profile's.
// Dropped from a profile's spawn: CURSOR_CONFIG_DIR, which would put its chats
// back in Personal's, and the variables that stand in for a login.
var Cursor = &Provider{
	ID: providers.Cursor,
	store: profilestore.Spec{
		Provider:    providers.Cursor,
		Label:       "Cursor CLI",
		HomeVar:     cursorHomeVar,
		PersonalDir: cursorBase,
		Own:         []string{cursorFolder},
		Override:    []string{cursorConfigDirVar, "CURSOR_API_KEY", "CURSOR_AUTH_TOKEN"},
		Pinned:      cursorPinned,
	},
	folder: cursorFolder,
	personalState: func() (string, error) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return providers.CursorConfigDir(home), nil
	},
	status: cursorStatus,
	login:  "login",
}

// Kiro is Kiro CLI's profiles. Its `kiro-cli/` under the data home holds the
// TUI it runs (bun, node, tui.js, a few hundred MB) beside data.sqlite3, which
// holds the login with the CLI's history and state, so only the database (and
// the lock its token refresh takes) is the profile's and the rest is linked.
// Its conversations are under ~/.kiro, which no profile moves, so they are
// every profile's.
var Kiro = &Provider{
	ID: providers.Kiro,
	store: profilestore.Spec{
		Provider:    providers.Kiro,
		Label:       "Kiro CLI",
		HomeVar:     dataHomeVar,
		PersonalDir: dataHome,
		Own: []string{
			filepath.Join("kiro-cli", "data.sqlite3"),
			filepath.Join("kiro-cli", "data.sqlite3-wal"),
			filepath.Join("kiro-cli", "data.sqlite3-shm"),
			filepath.Join("kiro-cli", "data.sqlite3-journal"),
			filepath.Join("kiro-cli", ".refresh.lock"),
		},
		Override: []string{"KIRO_API_KEY"},
	},
	folder:        "kiro-cli",
	personalState: func() (string, error) { return under(dataHome, "kiro-cli") },
	status:        kiroStatus,
	login:         "login",
}

// Providers is every provider with link-home profiles on this platform.
func Providers() []*Provider { return providersHere }

// Of is the provider with id, nil when it has no profiles here.
func Of(id string) *Provider {
	for _, p := range Providers() {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func configHome() (string, error) { return baseDir(configHomeVar, ".config") }

func dataHome() (string, error) { return baseDir(dataHomeVar, filepath.Join(".local", "share")) }

// baseDir is an XDG base directory: the variable when it is absolute, which is
// all the specification lets it be, else its default under the home.
func baseDir(name, fallback string) (string, error) {
	if dir := os.Getenv(name); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, fallback), nil
}

func under(base func() (string, error), name string) (string, error) {
	dir, err := base()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// ConfigDir is a profile's base directory, Personal's included. A profile that
// does not exist answers ErrProfileGone.
func (p *Provider) ConfigDir(slug string) (string, error) { return p.store.ConfigDir(slug) }

// List is every profile, Personal first and the rest in the order they were
// created.
func (p *Provider) List() ([]Profile, error) { return p.store.List() }

// Create makes a profile named name, its link home laid out against Personal's
// base directory. It does not log in.
func (p *Provider) Create(name string) (Profile, error) { return p.store.Create(name) }

// Update changes a profile's name, color or image. The slug and the directory
// stay.
func (p *Provider) Update(slug string, u profilestore.Update) (Profile, error) {
	return p.store.Update(slug, u)
}

// Delete removes a profile's link home and the login in it. The links go and
// what they point at stays.
func (p *Provider) Delete(slug string) error { return p.store.Delete(slug) }

// EnsureShared brings a profile's link home in step with Personal's base
// directory before a spawn into it.
func (p *Provider) EnsureShared(slug string) error { return p.store.EnsureShared(slug) }

// Env is base with the provider's XDG variable naming the link home dir and
// the variables that would override the profile's login removed.
func (p *Provider) Env(base []string, dir string) []string { return p.store.Env(base, dir) }

// State is a profile with the directory its provider keeps conversations in.
type State struct {
	Profile
	Dir string
}

// States is every profile with its state directory, Personal first. A reader
// of the provider's conversations scans all of them.
func (p *Provider) States() []State {
	var out []State
	for _, profile := range p.store.All() {
		dir, err := p.stateOf(profile.Dir)
		if err != nil {
			continue
		}
		out = append(out, State{Profile: profile, Dir: dir})
	}
	return out
}

// stateOf is the state directory of the profile whose base directory is home.
func (p *Provider) stateOf(home string) (string, error) {
	personal, err := p.store.PersonalDir()
	if err != nil {
		return "", err
	}
	if filepath.Clean(home) == filepath.Clean(personal) {
		return p.personalState()
	}
	return filepath.Join(home, p.folder), nil
}

// Owner is the profile holding a conversation and where locate found it under
// that profile's state directory.
func (p *Provider) Owner(conversationID string, locate func(stateDir, conversationID string) (string, bool)) (Profile, string, bool) {
	for _, s := range p.States() {
		if path, ok := locate(s.Dir, conversationID); ok {
			return s.Profile, path, true
		}
	}
	return Profile{}, "", false
}

// ErrNoMove is MoveConversation's answer for a provider whose conversations
// cannot change profile.
var ErrNoMove = errors.New("this provider's conversations cannot move to another profile")
