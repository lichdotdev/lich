package library

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// CardItems are the MCP servers and plugins a session of provider loads from
// the user's own setup, which a subagent card may leave out one by one
// (api.SubagentCards): the servers its config holds, the library's switched on
// for it in any profile, and its plugins switched on in any profile. lich's
// own are never among them. Their On is the caller's.
func (s *Service) CardItems(provider string) []api.SubagentCardItem {
	var out []api.SubagentCardItem
	seen := map[string]bool{}
	add := func(it api.SubagentCardItem) {
		if !seen[it.ID] {
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	for _, f := range s.foundServers() {
		if f.provider == provider {
			add(api.SubagentCardItem{ID: CardMCPID(f.name), Kind: api.SubagentBringMCP, Name: f.name, Origin: filepath.Base(f.doc.path)})
		}
	}
	if servers, err := s.libraryServers(); err == nil {
		for name := range servers {
			if s.onInAnyProfile(api.LibraryMCP, name, provider) {
				add(api.SubagentCardItem{ID: CardMCPID(name), Kind: api.SubagentBringMCP, Name: name, Origin: api.SubagentOriginLibrary})
			}
		}
	}
	for _, it := range s.plugins("") {
		if pluginOnFor(it, provider) {
			name, marketplace, _ := strings.Cut(it.Name, "@")
			add(api.SubagentCardItem{ID: CardPluginID(it.Name), Kind: api.SubagentBringPlugin, Name: name, Origin: marketplace})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == api.SubagentBringMCP
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// CardMCPID and CardPluginID are the api.SubagentCardItem IDs of an MCP server
// and of a plugin by its name@marketplace.
func CardMCPID(name string) string  { return api.SubagentBringMCP + ":" + name }
func CardPluginID(id string) string { return api.SubagentBringPlugin + ":" + id }

func (s *Service) onInAnyProfile(kind, name, provider string) bool {
	if s.On(kind, name, provider, "") {
		return true
	}
	return slices.ContainsFunc(s.profilesOf(provider), func(p api.Profile) bool { return s.On(kind, name, provider, p.Slug) })
}

func pluginOnFor(it api.LibraryItem, provider string) bool {
	return slices.ContainsFunc(it.Switches, func(sw api.LibrarySwitch) bool { return sw.Provider == provider && sw.On })
}
