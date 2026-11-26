package library

import (
	"path/filepath"
	"slices"
	"sort"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// A project's own MCP servers: the files in it each provider loads them from,
// and Claude Code's local scope, kept for the project in ~/.claude.json. lich
// shows them in the project's scope and never moves one: most of these files
// are committed, and the rest belong to that project alone. Antigravity and
// oh-my-pi read no project file lich knows of yet (docs/ceilings.md).

// projectDocs are the documents in project provider reads MCP servers from.
func projectDocs(provider, project string) []mcpDoc {
	switch provider {
	case providers.Claude:
		return []mcpDoc{{path: filepath.Join(project, ".mcp.json"), key: mcpServers}}
	case providers.Codex:
		return []mcpDoc{{path: filepath.Join(project, ".codex", "config.toml"), shape: shapeCodex}}
	case providers.OpenCode:
		return opencodeDocs(project)
	case providers.Crush:
		return []mcpDoc{
			{path: filepath.Join(project, "crush.json"), key: "mcp", shape: shapeCrush},
			{path: filepath.Join(project, ".crush.json"), key: "mcp", shape: shapeCrush},
		}
	case providers.Cursor:
		return []mcpDoc{{path: filepath.Join(project, ".cursor", "mcp.json"), key: mcpServers}}
	case providers.Kiro:
		return []mcpDoc{{path: filepath.Join(project, ".kiro", "settings", "mcp.json"), key: mcpServers}}
	}
	return nil
}

// projectServers are the MCP servers project's own files declare, one row per
// name, each with the files it is in: relative to the project, or ~/.claude.json
// for Claude Code's local scope.
func (s *Service) projectServers(project string) []api.LibraryFound {
	byName := map[string]*api.LibraryFound{}
	add := func(f foundServer, provider, path string) {
		row := byName[f.name]
		if row == nil {
			row = &api.LibraryFound{Kind: api.LibraryMCP, Name: f.name, Description: f.server.commandLine(), Transport: f.server.transport()}
			byName[f.name] = row
		}
		if !slices.Contains(row.Providers, provider) {
			row.Providers = append(row.Providers, provider)
		}
		if !slices.Contains(row.Paths, path) {
			row.Paths = append(row.Paths, path)
		}
	}
	for _, p := range providers.Registry {
		for _, doc := range projectDocs(p.ID, project) {
			rel, _ := filepath.Rel(project, doc.path)
			for _, f := range docServers(doc) {
				add(f, p.ID, filepath.ToSlash(rel))
			}
		}
	}
	for _, f := range claudeLocalServers(project) {
		add(f, providers.Claude, "~/.claude.json")
	}
	out := make([]api.LibraryFound, 0, len(byName))
	for _, row := range byName {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// claudeLocalServers are Claude Code's local-scope servers for project, under
// its entry of projects in Personal's ~/.claude.json.
func claudeLocalServers(project string) []foundServer {
	path, err := claudeprofile.GlobalFile("")
	if err != nil {
		return nil
	}
	config, err := readDoc(path)
	if err != nil {
		return nil
	}
	projects, _ := config["projects"].(map[string]any)
	entry, _ := projects[project].(map[string]any)
	entries, _ := entry[mcpServers].(map[string]any)
	var out []foundServer
	for name, raw := range entries {
		if sv, ok := serverOf(raw, shapeServers); ok {
			out = append(out, foundServer{name: name, server: sv})
		}
	}
	return out
}
