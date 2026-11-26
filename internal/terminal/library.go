package terminal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sort"

	"github.com/lichdotdev/lich/internal/library"
	"github.com/lichdotdev/lich/internal/providers"
)

// How a session is handed the items lich's library has on for its provider
// (internal/library): the routes are the ones the plugin takes (plugin.go),
// and the MCP servers ride beside lich's own on the providers that take one
// at spawn.

// Librarian works out what a spawn is handed from the library.
type Librarian interface {
	Deliver(provider, profile, profileDir string) library.Delivery
}

// SetLibrary wires the library spawns read. Startup wiring, before any spawn.
func (s *Service) SetLibrary(l Librarian) { s.library = l }

// libraryFor is what kind's spawn in profile is handed, nothing without a
// library.
func (s *Service) libraryFor(kind, profile, profileDir string) library.Delivery {
	if s.library == nil || !providers.Known(kind) {
		return library.Delivery{}
	}
	return s.library.Deliver(kind, profile, profileDir)
}

const ompConfigFlag = "--config"

// librarySkillArgs are the arguments that load the library's skills: a second
// plugin folder for Claude Code and Cursor CLI, an overlay for oh-my-pi.
func librarySkillArgs(kind string, lib library.Delivery) []string {
	switch kind {
	case providers.Claude, providers.Cursor:
		if lib.PluginDir != "" {
			return []string{pluginDirFlag + lib.PluginDir}
		}
	case providers.OMP:
		if lib.OMPConfig != "" {
			return []string{ompConfigFlag, lib.OMPConfig}
		}
	}
	return nil
}

// crushSkillsVar replaces Crush's whole global skill search, so the folder it
// names holds Crush's own skills beside the library's (library.buildCrush).
const crushSkillsVar = "CRUSH_SKILLS_DIR"

// libraryEnv hands opencode the skills folder and servers through its inline
// config, and Crush its composed skills folder.
func libraryEnv(env []string, kind string, lib library.Delivery) []string {
	switch kind {
	case providers.Crush:
		if lib.CrushSkills != "" {
			return append(env, crushSkillsVar+"="+lib.CrushSkills)
		}
	case providers.OpenCode:
		if lib.SkillsDir == "" && len(lib.MCP) == 0 {
			return env
		}
		extended, err := extendInlineConfig(env, opencodeConfigContentVar, func(config map[string]any) error {
			if lib.SkillsDir != "" {
				skills, err := objectAt(config, "skills")
				if err != nil {
					return err
				}
				paths, _ := skills["paths"].([]any)
				skills["paths"] = append(paths, lib.SkillsDir)
			}
			if len(lib.MCP) == 0 {
				return nil
			}
			servers, err := objectAt(config, opencodeMCPKey)
			if err == nil {
				for name, sv := range lib.MCP {
					servers[name] = opencodeServer(sv)
				}
			}
			return err
		})
		if err != nil {
			slog.Warn("library skipped", "provider", kind, "err", err)
			return env
		}
		return extended
	}
	return env
}

// opencodeServer is a library server in opencode's own shape.
func opencodeServer(sv library.Server) map[string]any {
	if sv.URL != "" {
		return map[string]any{"type": "remote", "url": sv.URL}
	}
	out := map[string]any{"type": "local", "command": append([]string{sv.Command}, sv.Args...)}
	if len(sv.Env) > 0 {
		out["environment"] = sv.Env
	}
	return out
}

// codexKey is a server name Codex's -c dotted path takes as written.
var codexKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// libraryMCPArgs adds the library's servers to mcp, the arguments that hand
// lich's own: one more JSON document on Claude Code's --mcp-config, -c
// overrides on Codex's. A name Codex's dotted path cannot carry is left out.
func libraryMCPArgs(kind string, mcp []string, servers map[string]library.Server) []string {
	if len(servers) == 0 {
		return mcp
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	switch kind {
	case providers.Claude:
		doc := map[string]any{}
		for _, name := range names {
			sv := servers[name]
			if sv.URL != "" && sv.Type == "" {
				sv.Type = "http"
			}
			doc[name] = sv
		}
		raw, err := json.Marshal(map[string]any{"mcpServers": doc})
		if err != nil {
			return mcp
		}
		if len(mcp) == 0 {
			return []string{claudeMCPFlag, string(raw)}
		}
		return append(mcp, string(raw))
	case providers.Codex:
		for _, name := range names {
			if !codexKey.MatchString(name) {
				slog.Warn("library: an MCP server name Codex cannot take", "name", name)
				continue
			}
			mcp = append(mcp, codexServerArgs(name, servers[name])...)
		}
	}
	return mcp
}

// codexServerArgs are the -c overrides of one server in Codex's mcp_servers
// table.
func codexServerArgs(name string, sv library.Server) []string {
	key := "mcp_servers." + name
	if sv.URL != "" {
		return []string{codexConfigFlag, fmt.Sprintf("%s.url=%q", key, sv.URL)}
	}
	args, _ := json.Marshal(sv.Args)
	out := []string{
		codexConfigFlag, fmt.Sprintf("%s.command=%q", key, sv.Command),
		codexConfigFlag, fmt.Sprintf("%s.args=%s", key, args),
	}
	for k, v := range sv.Env {
		if codexKey.MatchString(k) {
			out = append(out, codexConfigFlag, fmt.Sprintf("%s.env.%s=%q", key, k, v))
		}
	}
	return out
}
