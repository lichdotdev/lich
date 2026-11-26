package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// The Cursor CLI side. Its sessions load lich's own copy of the plugin with
// `--plugin-dir`, in the Claude Code plugin format it reads, and run its hooks
// (measured on 2026.11.19), so the version reported for Cursor is that copy's.
// Cursor also runs every plugin installed in Claude Code, which is why lich
// takes its own install out of there (migrateClaude): left in, every hook would
// report twice.
//
// What a plugin does not carry is the MCP registration: Cursor takes no server
// on its command line, and a Claude Code plugin declares none. That is the one
// file this writes — an `mcpServers` document under `~/.cursor`, merged into
// whatever is already there, the same trade omp's makes.

const (
	// cursorMCPFile is the document Cursor reads MCP servers from. It sits under
	// `~/.cursor` — resolved off the home with no variable in the way, unlike
	// the config directory that holds the credentials and the chats.
	cursorMCPFile = "mcp.json"
)

// cursorInstall readies the plugin Cursor sessions load and registers lich's
// MCP server with Cursor.
func (s *Service) cursorInstall() error {
	if err := s.installBundle(); err != nil {
		return err
	}
	// Merged before anything is written, for the reason omp's install gives: the
	// only way this step fails is a document lich must not replace.
	path, err := cursorMCPPath()
	if err != nil {
		return err
	}
	lichBin := s.lichBin()
	if lichBin == "" {
		return fmt.Errorf("lich cannot resolve its own binary to register with %s",
			providerName(providers.Cursor))
	}
	return writeMCPDocument(path, lichBin)
}

// cursorInstalledVersion is the version of the plugin Cursor sessions load,
// and only once lich's server is registered with Cursor: the reports come from
// that plugin and the tools from this registration, so a session has the
// plugin only when both are there. ("", false) when either half is missing.
func (s *Service) cursorInstalledVersion() (string, bool) {
	if !cursorRegistered() {
		return "", false
	}
	return bundleVersion()
}

// cursorRegistered reports whether lich's own server is in Cursor's MCP
// document. False for every failure — no file, a document lich cannot parse, an
// entry under another name — which reads as "not installed" and is what the
// install overwrites.
func cursorRegistered() bool {
	path, err := cursorMCPPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var config struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}
	_, ok := config.Servers[relay.MCPServerName]
	return ok
}

// cursorMCPPath is `~/.cursor/mcp.json`. Cursor reads this one off the home
// directly: $CURSOR_CONFIG_DIR and $XDG_CONFIG_HOME move the credentials and the
// chats, and leave this file where it is (measured on 2026.08.11).
func cursorMCPPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cursor", cursorMCPFile), nil
}
