package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// List answers the library, what the providers hold beside it and lich's own
// plugin, over RPC.
func (s *Service) List(_ context.Context, _ api.None) (api.Library, error) {
	out := api.Library{Skills: []api.LibraryItem{}, MCP: []api.LibraryItem{}, Found: s.found(), Plugin: s.plugin()}
	for _, p := range providers.Registry {
		out.Providers = append(out.Providers, api.LibraryProvider{
			ID: p.ID, Name: p.Name, Profiles: s.profilesOf(p.ID), NoSkills: noSkills(p.ID), NoMCP: noMCP(p.ID),
		})
	}
	skills, err := s.librarySkills()
	if err != nil {
		return out, err
	}
	for _, sk := range skills {
		out.Skills = append(out.Skills, api.LibraryItem{
			Kind: api.LibrarySkill, Name: sk.name, Description: sk.description, Switches: s.switches(api.LibrarySkill, sk.name, out.Providers),
		})
	}
	servers, err := s.libraryServers()
	if err != nil {
		return out, err
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		sv := servers[name]
		out.MCP = append(out.MCP, api.LibraryItem{
			Kind: api.LibraryMCP, Name: name, Command: sv.commandLine(), Transport: sv.transport(),
			Switches: s.switches(api.LibraryMCP, name, out.Providers),
		})
	}
	return out, nil
}

// switches are an item's stored switches, the providers' own and each of
// their profiles'.
func (s *Service) switches(kind, name string, list []api.LibraryProvider) []api.LibrarySwitch {
	var out []api.LibrarySwitch
	for _, p := range list {
		out = append(out, api.LibrarySwitch{Provider: p.ID, On: s.get(switchKey(kind, name, p.ID, "")) == switchOn})
		for _, prof := range p.Profiles {
			out = append(out, api.LibrarySwitch{Provider: p.ID, Profile: prof.Slug, On: s.get(switchKey(kind, name, p.ID, prof.Slug)) != switchOff})
		}
	}
	return out
}

// profilesOf are provider's profiles when it has more than Personal; a
// provider with one has nothing to choose between.
func (s *Service) profilesOf(provider string) []api.Profile {
	if s.profiles == nil {
		return nil
	}
	list := s.profiles(provider)
	if len(list) < 2 {
		return nil
	}
	return list
}

// Set turns an item on or off for a provider, or one of its profiles, over
// RPC; it reaches the provider's next session.
func (s *Service) Set(_ context.Context, opts api.LibrarySwitchOptions) (api.None, error) {
	if !providers.Known(opts.Provider) {
		return api.None{}, fmt.Errorf("no provider %q", opts.Provider)
	}
	if opts.Kind != api.LibrarySkill && opts.Kind != api.LibraryMCP {
		return api.None{}, fmt.Errorf("no library item of kind %q", opts.Kind)
	}
	value := switchOff
	if opts.On {
		value = switchOn
	}
	if err := s.set(switchKey(opts.Kind, opts.Name, opts.Provider, opts.Profile), value); err != nil {
		return api.None{}, err
	}
	s.changed()
	return api.None{}, nil
}

// noSkills is why provider cannot take a library skill, "" when it can.
func noSkills(string) string { return "" }

// noMCP is why provider cannot take a library MCP server, "" when it can:
// Antigravity, Crush and Kiro CLI keep servers behind their own CLI or in a
// block lich writes for its own server, which this round does not write into.
func noMCP(provider string) string {
	switch provider {
	case providers.Antigravity, providers.Crush, providers.Kiro:
		return api.LibraryNotWritten
	}
	return ""
}

// plugin is lich's own plugin as the Plugins screen shows it: the release
// sessions run and what it brings, its own MCP server included.
func (s *Service) plugin() api.LibraryPlugin {
	if s.pluginDir == nil {
		return api.LibraryPlugin{}
	}
	dir := s.pluginDir()
	if dir == "" {
		return api.LibraryPlugin{}
	}
	out := api.LibraryPlugin{Version: filepath.Base(dir), Skills: len(readSkills(filepath.Join(dir, skillsDir))), MCP: 1}
	data, err := os.ReadFile(filepath.Join(dir, "hooks", "hooks.json"))
	if err != nil {
		return out
	}
	var doc struct {
		Hooks map[string][]struct {
			Hooks []json.RawMessage `json:"hooks"`
		} `json:"hooks"`
	}
	if json.Unmarshal(data, &doc) == nil {
		for _, entries := range doc.Hooks {
			for _, e := range entries {
				out.Hooks += len(e.Hooks)
			}
		}
	}
	return out
}
