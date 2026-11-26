// Package library is lich's own library of skills and MCP servers: what lich
// holds in <config>/lich/library, what each provider has installed in its own
// config that is not there yet (found.go), moving one in, turning each item on
// or off per provider and per profile, and handing the ones on to a session
// (deliver.go).
//
// The library is shared by every profile; the switches live in lich.db, one
// settings key per item, provider and profile. A switch never stored is off: an
// item reaches only the providers it was moved in from until someone turns it
// on elsewhere.
package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// Settings is where the switches are kept: the store's global settings.
type Settings interface {
	GetSetting(ctx context.Context, opts api.GetSettingOptions) (string, error)
	SetSetting(ctx context.Context, opts api.SetSettingOptions) (api.None, error)
}

// Service is the library over one config directory.
type Service struct {
	settings Settings
	emit     func(name string, data any)
	// configDir is os.UserConfigDir, and home os.UserHomeDir, swapped by tests.
	configDir func() (string, error)
	home      func() (string, error)
	// profiles lists a provider's profiles, Personal first, nil for one with
	// none.
	profiles func(provider string) []api.Profile
	// pluginDir names lich's own unpacked plugin release (agentplugin.BundleDir).
	pluginDir func() string

	mu sync.Mutex // one write to the library or a provider's config at a time
}

// New returns the library, reporting changes through emit.
func New(settings Settings, emit func(name string, data any), profiles func(string) []api.Profile, pluginDir func() string) *Service {
	return &Service{
		settings: settings, emit: emit, profiles: profiles, pluginDir: pluginDir,
		configDir: os.UserConfigDir, home: os.UserHomeDir,
	}
}

// libraryDir is <config>/lich/library.
func (s *Service) libraryDir() (string, error) {
	dir, err := s.configDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return filepath.Join(dir, "lich", "library"), nil
}

const (
	skillsDir   = "skills"
	mcpFile     = "mcp.json"
	skillFile   = "SKILL.md"
	mcpServers  = "mcpServers"
	switchOn    = "1"
	switchOff   = "0"
	keyPrefix   = "library."
	seenPrefix  = "library.seen."
	movedSuffix = ".lich-moved"
)

// Server is one MCP server as the library keeps it.
type Server struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
}

// transport is a server's kind of connection as the screens name it.
func (sv Server) transport() string {
	if sv.URL != "" {
		if sv.Type != "" && sv.Type != "stdio" {
			return sv.Type
		}
		return "http"
	}
	return "stdio"
}

// commandLine is the server's command, or its URL.
func (sv Server) commandLine() string {
	if sv.URL != "" {
		return sv.URL
	}
	return strings.TrimSpace(sv.Command + " " + strings.Join(sv.Args, " "))
}

// skill is one skill folder.
type skill struct {
	name, description, dir string
}

// librarySkills are the skills in the library, by name.
func (s *Service) librarySkills() ([]skill, error) {
	dir, err := s.libraryDir()
	if err != nil {
		return nil, err
	}
	return readSkills(filepath.Join(dir, skillsDir)), nil
}

// readSkills lists the skills in dir: each folder holding a SKILL.md, named by
// its frontmatter, or its folder when that names none.
func readSkills(dir string) []skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []skill
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(filepath.Join(path, skillFile))
		if err != nil {
			continue
		}
		name, description := frontmatter(string(data))
		if name == "" {
			name = e.Name()
		}
		out = append(out, skill{name: name, description: description, dir: path})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// frontmatter reads name and description off a SKILL.md's frontmatter.
func frontmatter(text string) (name, description string) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", ""
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = value
		case "description":
			description = value
		}
	}
	return name, description
}

// libraryServers are the MCP servers in the library.
func (s *Service) libraryServers() (map[string]Server, error) {
	dir, err := s.libraryDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, mcpFile))
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Server{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Servers map[string]Server `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Join(dir, mcpFile), err)
	}
	if doc.Servers == nil {
		doc.Servers = map[string]Server{}
	}
	return doc.Servers, nil
}

func (s *Service) writeServers(servers map[string]Server) error {
	dir, err := s.libraryDir()
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(map[string]any{mcpServers: servers}, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, mcpFile), append(body, '\n'))
}

// switchKey is where one switch is stored; profile "" is the provider's own.
func switchKey(kind, name, provider, profile string) string {
	return keyPrefix + kind + "." + name + "." + provider + "." + profile
}

func (s *Service) get(key string) string {
	value, err := s.settings.GetSetting(context.Background(), api.GetSettingOptions{Key: key})
	if err != nil {
		return ""
	}
	return value
}

func (s *Service) set(key, value string) error {
	_, err := s.settings.SetSetting(context.Background(), api.SetSettingOptions{Key: key, Value: value})
	return err
}

// On is whether item reaches provider's sessions in profile ("" for the
// provider as a whole): the profile's own switch once it was set, else the
// provider's, which a move in sets for every profile at once.
func (s *Service) On(kind, name, provider, profile string) bool {
	if profile != "" {
		switch s.get(switchKey(kind, name, provider, profile)) {
		case switchOn:
			return true
		case switchOff:
			return false
		}
	}
	return s.get(switchKey(kind, name, provider, "")) == switchOn
}

// changed tells the window the library or what the providers hold moved.
func (s *Service) changed() {
	if s.emit != nil {
		events.Send(s.emit, api.LibraryChangedEvent, api.None{})
	}
}

// seen are the found names of kind already shown once.
func (s *Service) seen(kind string) []string {
	var names []string
	_ = json.Unmarshal([]byte(s.get(seenPrefix+kind)), &names)
	return names
}

// Seen records every item of kind found now as seen, over RPC.
func (s *Service) Seen(_ context.Context, opts api.LibrarySeenOptions) (api.None, error) {
	var names []string
	for _, f := range s.found() {
		if f.Kind == opts.Kind {
			names = append(names, f.Name)
		}
	}
	slices.Sort(names)
	data, err := json.Marshal(names)
	if err != nil {
		return api.None{}, err
	}
	if err := s.set(seenPrefix+opts.Kind, string(data)); err != nil {
		return api.None{}, err
	}
	s.changed()
	return api.None{}, nil
}

// writeFile writes data through a temporary file in the same directory.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
