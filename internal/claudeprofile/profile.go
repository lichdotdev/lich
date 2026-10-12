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
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Personal is the slug of the default profile. It is reserved: no profile lich
// creates can take it, which is what lets a project setting name Personal
// explicitly where an empty value means "inherit the global one".
const Personal = "personal"

// ErrProfileGone is the error a spawn returns for a session born in a profile
// that was deleted since. The window matches it to offer a new session in the
// project's own profile instead.
var ErrProfileGone = errors.New("claude profile no longer exists")

// slugPattern is what a slug may be. It is checked on every path built from one,
// because a slug arrives over RPC and Delete removes the directory it names.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// maxSlugBase caps the part of a slug derived from the display name, leaving
// room under slugPattern's 40 for a "-N" suffix.
const maxSlugBase = 32

// metaFile is the sidecar lich keeps beside each profile directory, outside it:
// the directory itself is Claude Code's.
type metaFile struct {
	Name    string `json:"name"`
	Created int64  `json:"created"`
	// Copied lists the shared entries this profile holds as copies rather than
	// links, which Windows does without Developer Mode for files.
	Copied []string `json:"copied,omitempty"`
}

// Profile is one Claude Code account lich knows about.
type Profile struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	Personal bool   `json:"personal"`
	// Copied names the shared entries (settings.json, CLAUDE.md) this profile
	// holds as a copy, so edits made in one account no longer reach the other.
	Copied []string `json:"copied,omitempty"`
}

// Root is where lich keeps its Claude profiles. A `task dev` rig gets its own,
// the split the workspace database makes, so a dev build never deletes a real
// account.
func Root() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	name := "profiles"
	if os.Getenv("LICH_DEV") != "" {
		name = "profiles-dev"
	}
	return filepath.Join(dir, "lich", name, "claude"), nil
}

// Dir is the CLAUDE_CONFIG_DIR of a profile lich created. It does not check
// that the profile exists; Exists does.
func Dir(slug string) (string, error) {
	if slug == Personal || !slugPattern.MatchString(slug) {
		return "", fmt.Errorf("invalid claude profile %q", slug)
	}
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, slug), nil
}

// PersonalDir is the default profile's config directory: what Claude Code
// resolves from lich's own environment.
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
func ConfigDir(slug string) (string, error) {
	if IsPersonal(slug) {
		return PersonalDir()
	}
	dir, err := Dir(slug)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(metaPath(dir)); err != nil {
		return "", fmt.Errorf("%w: %q was deleted", ErrProfileGone, slug)
	}
	return dir, nil
}

// IsPersonal reports whether slug names the default profile. Empty does too: it
// is what every session row born before profiles carries.
func IsPersonal(slug string) bool { return slug == "" || slug == Personal }

func metaPath(dir string) string { return dir + ".json" }

func readMeta(dir string) (metaFile, error) {
	var m metaFile
	data, err := os.ReadFile(metaPath(dir))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("read %s: %w", metaPath(dir), err)
	}
	return m, nil
}

func writeMeta(dir string, m metaFile) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(metaPath(dir), data, 0o600)
}

// List is every profile, Personal first and the rest in the order they were
// created. A sidecar that does not parse is skipped rather than failing the
// list: one broken profile must not hide the others.
func List() ([]Profile, error) {
	personal, err := PersonalDir()
	if err != nil {
		return nil, err
	}
	out := []Profile{{Slug: Personal, Name: "Personal", Dir: personal, Personal: true}}
	root, err := Root()
	if err != nil {
		return nil, err
	}
	metas, _ := filepath.Glob(filepath.Join(root, "*.json"))
	type created struct {
		p  Profile
		at int64
	}
	var found []created
	for _, path := range metas {
		dir := strings.TrimSuffix(path, ".json")
		slug := filepath.Base(dir)
		if !slugPattern.MatchString(slug) || slug == Personal {
			continue
		}
		m, err := readMeta(dir)
		if err != nil {
			continue
		}
		found = append(found, created{Profile{Slug: slug, Name: m.Name, Dir: dir, Copied: m.Copied}, m.Created})
	}
	slices.SortStableFunc(found, func(a, b created) int { return cmp.Compare(a.at, b.at) })
	for _, c := range found {
		out = append(out, c.p)
	}
	return out, nil
}

// ConfigDirs is every profile's config directory keyed by slug, Personal first.
// Every reader of Claude Code's transcripts scans all of them: a conversation
// lives in exactly one, and which one is what says the profile it belongs to.
func ConfigDirs() []Profile {
	profiles, err := List()
	if err != nil {
		return nil
	}
	return profiles
}

// slugFor derives a free slug from a display name.
func slugFor(name, root string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	base := strings.Trim(b.String(), "-")
	if len(base) > maxSlugBase {
		base = strings.Trim(base[:maxSlugBase], "-")
	}
	if base == "" {
		base = "profile"
	}
	slug := base
	for n := 2; slug == Personal || taken(root, slug); n++ {
		slug = base + "-" + strconv.Itoa(n)
	}
	return slug
}

func taken(root, slug string) bool {
	for _, p := range []string{filepath.Join(root, slug), filepath.Join(root, slug+".json")} {
		if _, err := os.Lstat(p); err == nil {
			return true
		}
	}
	return false
}

// Create makes a profile named name: its directory, laid out against Personal
// (layout.go), and its sidecar, written last so a half-built directory is never
// listed. It does not log in.
func Create(name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, errors.New("a profile needs a name")
	}
	root, err := Root()
	if err != nil {
		return Profile{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Profile{}, fmt.Errorf("create %s: %w", root, err)
	}
	slug := slugFor(name, root)
	dir := filepath.Join(root, slug)
	copied, err := build(dir)
	if err != nil {
		_ = remove(dir)
		return Profile{}, err
	}
	m := metaFile{Name: name, Created: time.Now().UnixNano(), Copied: copied}
	if err := writeMeta(dir, m); err != nil {
		_ = remove(dir)
		return Profile{}, err
	}
	return Profile{Slug: slug, Name: name, Dir: dir, Copied: copied}, nil
}

// Rename changes a profile's display name. The slug and the directory stay.
func Rename(slug, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("a profile needs a name")
	}
	dir, err := ConfigDir(slug)
	if err != nil {
		return err
	}
	if IsPersonal(slug) {
		return errors.New("the Personal profile cannot be renamed")
	}
	m, err := readMeta(dir)
	if err != nil {
		return err
	}
	m.Name = name
	return writeMeta(dir, m)
}

// Delete removes a profile's directory and its login with it. Personal cannot
// be deleted.
func Delete(slug string) error {
	if IsPersonal(slug) {
		return errors.New("the Personal profile cannot be deleted")
	}
	dir, err := ConfigDir(slug)
	if err != nil {
		return err
	}
	if err := os.Remove(metaPath(dir)); err != nil {
		return fmt.Errorf("delete profile %q: %w", slug, err)
	}
	return remove(dir)
}

// remove deletes a profile directory, unlinking the shared entries first so no
// removal can ever walk through a link into Personal's own files.
func remove(dir string) error {
	for _, name := range append(sharedEntries, sharedPluginEntries...) {
		path := filepath.Join(dir, name)
		if isLink(path) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("unlink %s: %w", path, err)
			}
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete %s: %w", dir, err)
	}
	return nil
}

// tokenVars are the variables that would override a profile's own login: a
// spawn into a profile drops them, or the account in its directory is never the
// one spent. The secure-storage override goes too, since on macOS it would point
// every profile at one Keychain item.
var tokenVars = []string{
	"CLAUDE_CODE_OAUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN",
	"CLAUDE_SECURESTORAGE_CONFIG_DIR", "CLAUDE_CONFIG_DIR",
}

// Env is base with a profile's directory as CLAUDE_CONFIG_DIR and the variables
// that would override its login removed.
func Env(base []string, dir string) []string {
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if slices.Contains(tokenVars, key) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "CLAUDE_CONFIG_DIR="+dir)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
