// Package profilestore keeps more than one login of a provider on one machine.
// A profile is a state directory of its own holding its own login, so every
// account runs with its full login rather than an inference-only token. The
// providers that have profiles (internal/claudeprofile, internal/codexprofile,
// internal/ompprofile, internal/xdgprofile) describe themselves with a Spec;
// this package is the rest: the slug and the sidecar a profile is listed from,
// the layout of its directory against the default profile (Personal), and the
// move of a conversation between two of them.
//
// A profile directory is never moved once it exists, and renaming a profile
// changes its display name only: the slug names the directory, and a provider
// may key a login on the directory string (Claude Code's macOS Keychain entry
// and Codex's keyring store do).
package profilestore

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// Personal is the slug of the default profile. It is reserved: no profile lich
// creates can take it, which is what lets a project setting name Personal
// explicitly where an empty value means "inherit the global one".
const Personal = "personal"

// ErrGone is the error a spawn returns for a session born in a profile that was
// deleted since. The window matches it to offer a new session in the project's
// own profile instead.
var ErrGone = errors.New("profile no longer exists")

// slugPattern is what a slug may be. It is checked on every path built from one,
// because a slug arrives over RPC and Delete removes the directory it names.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// maxSlugBase caps the part of a slug derived from the display name, leaving
// room under slugPattern's 40 for a "-N" suffix.
const maxSlugBase = 32

// Spec is what differs between providers. Everything else about a profile is
// the same, so a Spec is a value rather than an interface.
type Spec struct {
	// Provider names the provider in messages and is the subdirectory of lich's
	// profiles root its sidecars live in ("claude").
	Provider string
	// Label is the provider as a person calls it ("Claude Code").
	Label string
	// HomeVar is the environment variable the provider reads its state from.
	HomeVar string
	// PersonalDir is the default profile's state directory: what the provider
	// resolves from lich's own environment.
	PersonalDir func() (string, error)
	// DirOf is where a profile's state directory is. Nil puts it beside its
	// sidecar, under lich's profiles root. It is given a slug that already
	// matched slugPattern.
	DirOf func(slug string) (string, error)
	// SharedFiles and Shared are the user's own content, linked from every
	// profile to Personal so an edit made under one account is there under all
	// of them: files, which Windows can only link with Developer Mode and
	// otherwise copies, and directories. Only entries Personal has are linked;
	// EnsureShared links one created later.
	SharedFiles, Shared []string
	// Own, when set, makes the profile directory a link home (linkhome.go):
	// a mirror of Personal's directory in which every entry is a link except
	// these paths, relative to it, which are the profile's own. SharedFiles and
	// Shared are then unused: everything else is shared.
	Own []string
	// Downloads are directories Personal keeps its plugin downloads in, created
	// there when absent and linked, so a profile does not fetch them again.
	Downloads []string
	// PersonalPath is where Personal keeps a linked entry, given its state
	// directory and the entry's path relative to a profile's. Nil is the same
	// relative path under Personal's: it differs only where the provider reads a
	// variable that moves part of its state elsewhere.
	PersonalPath func(personal, name string) string
	// Override are the variables, besides HomeVar, that would override a
	// profile's own login: a spawn into a profile drops them, or the account in
	// its directory is never the one spent.
	Override []string
	// Pinned are variables besides HomeVar that a spawn into a profile sets,
	// each to a path under the profile's directory: state HomeVar alone does
	// not move (Cursor CLI's config dir on Windows). Only for a provider whose
	// HomeVar value is that directory.
	Pinned []PinnedVar
	// Reserved are slugs a profile cannot take because the provider reads them
	// as something else ("default" selects oh-my-pi's default profile).
	Reserved []string
	// Seed runs once a profile is linked, for what is not a link: Claude Code's
	// global config and plugin state are files of their own.
	Seed func(personal, dir string) error
	// Sync runs after Seed and again before every spawn into the profile
	// (EnsureShared), for a file the profile holds as its own but rebuilds from
	// Personal's: Codex's config.toml, whose plugin set is the profile's.
	Sync func(personal, dir string) error
}

// PinnedVar is one of Spec.Pinned: Name set to Rel under the profile's
// directory.
type PinnedVar struct{ Name, Rel string }

// metaFile is the sidecar lich keeps beside each profile, outside its
// directory: the directory itself is the provider's.
type metaFile struct {
	Name    string `json:"name"`
	Created int64  `json:"created"`
	Color   string `json:"color,omitempty"`
	// Copied lists the shared entries this profile holds as copies rather than
	// links, which Windows does without Developer Mode for files.
	Copied []string `json:"copied,omitempty"`
}

type Profile = api.Profile

// IsPersonal reports whether slug names the default profile. Empty does too: it
// is what every session row born before profiles carries.
func IsPersonal(slug string) bool { return slug == "" || slug == Personal }

// Root is where lich keeps a provider's profile sidecars. A `task dev` rig gets
// its own, the split the workspace database makes, so a dev build never deletes
// a real account.
func (s Spec) Root() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	name := "profiles"
	if os.Getenv("LICH_DEV") != "" {
		name = "profiles-dev"
	}
	return filepath.Join(dir, "lich", name, s.Provider), nil
}

// Dir is the state directory of a profile lich created. It does not check that
// the profile exists; ConfigDir does.
func (s Spec) Dir(slug string) (string, error) {
	if slug == Personal || !slugPattern.MatchString(slug) || slices.Contains(s.Reserved, slug) {
		return "", fmt.Errorf("invalid %s profile %q", s.Provider, slug)
	}
	if s.DirOf != nil {
		return s.DirOf(slug)
	}
	root, err := s.Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, slug), nil
}

// ConfigDir is a profile's state directory, Personal's included. A profile that
// does not exist answers ErrGone.
func (s Spec) ConfigDir(slug string) (string, error) {
	if IsPersonal(slug) {
		return s.PersonalDir()
	}
	dir, err := s.Dir(slug)
	if err != nil {
		return "", err
	}
	meta, err := s.metaPath(slug)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(meta); err != nil {
		return "", fmt.Errorf("%s %w: %q was deleted", s.Provider, ErrGone, slug)
	}
	return dir, nil
}

func (s Spec) metaPath(slug string) (string, error) {
	root, err := s.Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, slug+".json"), nil
}

func (s Spec) readMeta(slug string) (metaFile, error) {
	var m metaFile
	path, err := s.metaPath(slug)
	if err != nil {
		return m, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("read %s: %w", path, err)
	}
	return m, nil
}

func (s Spec) writeMeta(slug string, m metaFile) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path, err := s.metaPath(slug)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, data, 0o600)
}

// List is every profile, Personal first and the rest in the order they were
// created. A sidecar that does not parse is skipped rather than failing the
// list: one broken profile must not hide the others.
func (s Spec) List() ([]Profile, error) {
	personal, err := s.PersonalDir()
	if err != nil {
		return nil, err
	}
	root, err := s.Root()
	if err != nil {
		return nil, err
	}
	out := []Profile{s.personal(personal)}
	// Globbed inside root, which may hold metacharacters of its own.
	metas, _ := fs.Glob(os.DirFS(root), "*.json")
	type created struct {
		p  Profile
		at int64
	}
	var found []created
	for _, name := range metas {
		slug := strings.TrimSuffix(name, ".json")
		dir, err := s.Dir(slug)
		if err != nil {
			continue
		}
		m, err := s.readMeta(slug)
		if err != nil {
			continue
		}
		p := Profile{Slug: slug, Name: m.Name, Dir: dir, Copied: m.Copied, Color: m.Color, Image: s.readImage(slug)}
		found = append(found, created{p, m.Created})
	}
	slices.SortStableFunc(found, func(a, b created) int { return cmp.Compare(a.at, b.at) })
	for _, c := range found {
		out = append(out, c.p)
	}
	return out, nil
}

// All is every profile with its state directory, Personal first. Every reader of
// a provider's transcripts scans all of them: a conversation lives in exactly
// one, and which one is what says the profile it belongs to.
func (s Spec) All() []Profile {
	profiles, err := s.List()
	if err != nil {
		return nil
	}
	return profiles
}

// slugFor derives a free slug from a display name.
func (s Spec) slugFor(name string) string {
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
	for n := 2; slug == Personal || slices.Contains(s.Reserved, slug) || s.taken(slug); n++ {
		slug = base + "-" + strconv.Itoa(n)
	}
	return slug
}

func (s Spec) taken(slug string) bool {
	dir, err := s.Dir(slug)
	if err != nil {
		return true
	}
	meta, err := s.metaPath(slug)
	if err != nil {
		return true
	}
	for _, p := range []string{dir, meta} {
		if _, err := os.Lstat(p); err == nil {
			return true
		}
	}
	return false
}

// Create makes a profile named name: its directory, laid out against Personal,
// and its sidecar, written last so a half-built directory is never listed. It
// does not log in.
func (s Spec) Create(name string) (Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Profile{}, errors.New("a profile needs a name")
	}
	root, err := s.Root()
	if err != nil {
		return Profile{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Profile{}, fmt.Errorf("create %s: %w", root, err)
	}
	slug := s.slugFor(name)
	dir, err := s.Dir(slug)
	if err != nil {
		return Profile{}, err
	}
	copied, err := s.build(dir)
	if err != nil {
		_ = s.remove(dir)
		return Profile{}, err
	}
	m := metaFile{Name: name, Created: time.Now().UnixNano(), Copied: copied}
	if err := s.writeMeta(slug, m); err != nil {
		_ = s.remove(dir)
		return Profile{}, err
	}
	return Profile{Slug: slug, Name: name, Dir: dir, Copied: copied}, nil
}

// Personal's display name and color are kept in a sidecar among the other
// profiles' rather than beside its own directory, whose ".json" neighbour may be
// the provider's own (~/.claude.json).
func (s Spec) personal(dir string) Profile {
	p := Profile{Slug: Personal, Name: "Personal", Dir: dir, Personal: true, Image: s.readImage(Personal)}
	if m, err := s.readMeta(Personal); err == nil {
		p.Color = m.Color
		if m.Name != "" {
			p.Name = m.Name
		}
	}
	return p
}

// Delete removes a profile's directory and its login with it. Personal cannot
// be deleted.
func (s Spec) Delete(slug string) error {
	if IsPersonal(slug) {
		return errors.New("the Personal profile cannot be deleted")
	}
	dir, err := s.ConfigDir(slug)
	if err != nil {
		return err
	}
	meta, err := s.metaPath(slug)
	if err != nil {
		return err
	}
	if err := os.Remove(meta); err != nil {
		return fmt.Errorf("delete profile %q: %w", slug, err)
	}
	if err := s.writeImage(slug, nil); err != nil {
		return err
	}
	return s.remove(dir)
}

// remove deletes a profile directory, unlinking the shared entries first so no
// removal can ever walk through a link into Personal's own files.
func (s Spec) remove(dir string) error {
	for _, name := range s.linked() {
		path := filepath.Join(dir, name)
		if IsLink(path) {
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

// Unset is every variable a spawn into a profile drops, HomeVar last.
func (s Spec) Unset() []string {
	return append(slices.Clone(s.Override), s.HomeVar)
}

// Env is base with HomeVar set to value and the variables that would override
// the profile's login removed. What value is depends on the provider: the
// directory itself, or the profile's name.
func (s Spec) Env(base []string, value string) []string {
	unset := s.Unset()
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		key, _, _ := strings.Cut(kv, "=")
		if slices.Contains(unset, key) {
			continue
		}
		out = append(out, kv)
	}
	for _, v := range s.assignments(value) {
		out = append(out, v[0]+"="+v[1])
	}
	return out
}

// assignments is every variable a spawn into the profile at value sets, as
// name and value: HomeVar, then Pinned.
func (s Spec) assignments(value string) [][2]string {
	out := [][2]string{{s.HomeVar, value}}
	for _, p := range s.Pinned {
		out = append(out, [2]string{p.Name, filepath.Join(value, p.Rel)})
	}
	return out
}

// WriteFileAtomic writes data to path through a temporary file and a rename, so
// a reader never sees it half written. Each call has a temporary file of its
// own: two spawns into one profile write the same files at once.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmp := f.Name()
	_, err = f.Write(data)
	err = errors.Join(err, f.Chmod(perm), f.Close())
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := replaceFile(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
