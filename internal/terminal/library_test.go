package terminal

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/library"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/subagent"
)

// Every provider is handed the library by the route it takes, and the ones
// that take nothing per session get nothing on their argv or environment.
func TestLibraryRoutesForEveryProvider(t *testing.T) {
	lib := library.Delivery{
		PluginDir: "/lib/b", SkillsDir: "/lib/b/skills", OMPConfig: "/lib/b/omp.yml", CrushSkills: "/lib/crush",
		MCP: map[string]library.Server{"postgres": {Command: "npx", Args: []string{"pg"}}},
	}
	for _, p := range providers.Registry {
		t.Run(p.ID, func(t *testing.T) {
			args := providerArgs(p.ID, "", "", "", "", "/usr/bin/lich", "", "", lib, subagent.Card{}, false, false, false, relay.RouteCards, prompt.English)
			joined := strings.Join(args, " ")
			env := strings.Join(libraryEnv(nil, p.ID, lib), " ")
			switch p.ID {
			case providers.Claude:
				i := slices.Index(args, claudeMCPFlag)
				if !strings.Contains(joined, "--plugin-dir=/lib/b") || i < 0 || len(args) != i+3 || !strings.Contains(args[i+2], `"postgres"`) {
					t.Errorf("args %v, want the plugin folder and a second --mcp-config document last", args)
				}
			case providers.Cursor:
				if !strings.Contains(joined, "--plugin-dir=/lib/b") {
					t.Errorf("args %v, want the plugin folder", args)
				}
			case providers.OMP:
				if !strings.Contains(joined, "--config /lib/b/omp.yml") {
					t.Errorf("args %v, want the overlay", args)
				}
			case providers.Codex:
				if !strings.Contains(joined, `mcp_servers.postgres.command="npx"`) || !strings.Contains(joined, `mcp_servers.postgres.args=["pg"]`) {
					t.Errorf("args %v, want the server as -c overrides", args)
				}
			case providers.OpenCode:
				var config map[string]any
				_ = json.Unmarshal([]byte(strings.TrimPrefix(env, opencodeConfigContentVar+"=")), &config)
				skills, _ := config["skills"].(map[string]any)
				servers, _ := config[opencodeMCPKey].(map[string]any)
				if paths, _ := skills["paths"].([]any); len(paths) != 1 || servers["postgres"] == nil {
					t.Errorf("inline config %v, want the skills folder and the server", config)
				}
			case providers.Crush:
				if env != crushSkillsVar+"=/lib/crush" {
					t.Errorf("env %q, want the composed skills folder", env)
				}
			}
			if p.ID != providers.OpenCode && p.ID != providers.Crush && env != "" {
				t.Errorf("env %q, want nothing", env)
			}
		})
	}
}

// A server name Codex's dotted -c path cannot carry is left out rather than
// handed as something else.
func TestLibraryMCPArgsLeavesOutANameCodexCannotTake(t *testing.T) {
	got := libraryMCPArgs(providers.Codex, nil, map[string]library.Server{"a.b": {Command: "x"}, "ok": {URL: "https://h/mcp"}})
	if want := []string{codexConfigFlag, `mcp_servers.ok.url="https://h/mcp"`}; !slices.Equal(got, want) {
		t.Errorf("args %v, want %v", got, want)
	}
}

// With no lich server to ride beside, Claude Code still gets the library's.
func TestLibraryMCPArgsOpensClaudesFlag(t *testing.T) {
	got := libraryMCPArgs(providers.Claude, nil, map[string]library.Server{"api": {URL: "https://h/mcp"}})
	if len(got) != 2 || got[0] != claudeMCPFlag || !strings.Contains(got[1], `"type":"http"`) {
		t.Errorf("args %v, want --mcp-config with the server typed http", got)
	}
}
