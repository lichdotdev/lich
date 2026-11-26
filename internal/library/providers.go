package library

import (
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/ompprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// Where each provider keeps the skills and MCP servers installed in its own
// config, which found.go reads and a move in turns off, and where lich writes
// into for the providers that take nothing per session (deliver.go).

// skillDirs are the global skill folders provider reads, Personal's.
func (s *Service) skillDirs(provider string) []string {
	home, err := s.home()
	if err != nil {
		return nil
	}
	switch provider {
	case providers.Claude:
		if dir, err := claudeprofile.PersonalDir(); err == nil {
			return []string{filepath.Join(dir, skillsDir)}
		}
	case providers.Codex:
		dirs := []string{filepath.Join(home, ".agents", skillsDir)}
		if dir, err := codexprofile.PersonalDir(); err == nil {
			dirs = append([]string{filepath.Join(dir, skillsDir)}, dirs...)
		}
		return dirs
	case providers.Antigravity:
		return []string{filepath.Join(home, ".gemini", skillsDir)}
	case providers.OpenCode:
		return []string{filepath.Join(xdgConfig(home), "opencode", skillsDir)}
	case providers.OMP:
		if dir, err := ompprofile.PersonalDir(); err == nil {
			return []string{filepath.Join(dir, skillsDir)}
		}
	case providers.Crush:
		return crushSkillDirs(home)
	case providers.Cursor:
		return []string{filepath.Join(home, ".cursor", skillsDir)}
	case providers.Kiro:
		return []string{filepath.Join(providers.KiroHome(home), skillsDir)}
	}
	return nil
}

// crushSkillDirs are the global folders Crush reads skills from, in the order
// its own search lists them (strings of 0.100.0; measured to make up its whole
// global set, 109 of 109).
func crushSkillDirs(home string) []string {
	return []string{
		filepath.Join(home, ".config", "crush", skillsDir),
		filepath.Join(home, ".agents", skillsDir),
		filepath.Join(home, ".claude", skillsDir),
		filepath.Join(home, ".cursor", skillsDir),
	}
}

// mcpDoc is one JSON document a provider reads MCP servers from: its path and
// the key holding them, with the shape it writes them in.
type mcpDoc struct {
	path, key string
	shape     docShape
}

// cliOwned are the providers whose MCP servers lich reads but only their own
// CLI removes: Claude Code rewrites ~/.claude.json while it runs, and Codex
// keeps them in TOML (removeServer).
func cliOwned(provider string) bool {
	return provider == providers.Claude || provider == providers.Codex
}

type docShape int

const (
	// shapeServers is the {"command", "args", "env", "url", "type"} shape
	// Cursor, oh-my-pi, Kiro CLI and Antigravity share.
	shapeServers docShape = iota
	// shapeOpencode is opencode's {"type": "local"|"remote", "command": [...],
	// "environment", "url"}.
	shapeOpencode
	// shapeCrush is Crush's {"type", "command", "args", "env", "url"}.
	shapeCrush
	// shapeCodex is a config.toml's [mcp_servers.<name>] tables.
	shapeCodex
)

// mcpDocs are the documents provider keeps its own user MCP servers in,
// Personal's.
func (s *Service) mcpDocs(provider string) []mcpDoc {
	home, err := s.home()
	if err != nil {
		return nil
	}
	switch provider {
	case providers.Claude:
		if path, err := claudeprofile.GlobalFile(""); err == nil {
			return []mcpDoc{{path: path, key: mcpServers}}
		}
	case providers.Codex:
		if dir, err := codexprofile.PersonalDir(); err == nil {
			return []mcpDoc{{path: filepath.Join(dir, "config.toml"), shape: shapeCodex}}
		}
	case providers.Antigravity:
		return []mcpDoc{{path: filepath.Join(home, ".gemini", "config", "mcp_config.json"), key: mcpServers}}
	case providers.OpenCode:
		return opencodeDocs(filepath.Join(xdgConfig(home), "opencode"))
	case providers.OMP:
		if dir, err := ompprofile.PersonalDir(); err == nil {
			return []mcpDoc{{path: filepath.Join(dir, "mcp.json"), key: mcpServers}}
		}
	case providers.Crush:
		return []mcpDoc{{path: filepath.Join(home, ".config", "crush", "crush.json"), key: "mcp", shape: shapeCrush}}
	case providers.Cursor:
		return []mcpDoc{{path: filepath.Join(home, ".cursor", "mcp.json"), key: mcpServers}}
	case providers.Kiro:
		return []mcpDoc{{path: filepath.Join(providers.KiroHome(home), "settings", "mcp.json"), key: mcpServers}}
	}
	return nil
}

// opencodeDocs are the two names opencode reads its config under in dir.
func opencodeDocs(dir string) []mcpDoc {
	return []mcpDoc{
		{path: filepath.Join(dir, "opencode.json"), key: "mcp", shape: shapeOpencode},
		{path: filepath.Join(dir, "opencode.jsonc"), key: "mcp", shape: shapeOpencode},
	}
}

// xdgConfig is $XDG_CONFIG_HOME, else ~/.config: the base opencode reads on
// every platform.
func xdgConfig(home string) string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".config")
}
