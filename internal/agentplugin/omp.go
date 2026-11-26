package agentplugin

import (
	"path/filepath"

	"github.com/lichdotdev/lich/internal/ompprofile"
)

// The oh-my-pi side. Its sessions load the extension from lich's own copy of
// the plugin with `-e` (bundle.go, internal/terminal), so the copy a lich of the
// past wrote into omp's extensions directory is removed: omp loads that
// directory beside `-e`, and would run both (measured on 19.1.1).
//
// The MCP registration is the part a session cannot be handed. omp takes no
// `--mcp-config` flag, and the only place it reads a server from is an
// `mcpServers` document beside the extensions — so lich writes one
// (mcpDocument, shared with Cursor CLI, which reads the same shape).

const (
	// OMPExtension is the extension's path inside the plugin, and ompFile the
	// name a lich of the past gave it in omp's extensions directory.
	OMPExtension     = "omp/lich.js"
	ompFile          = "lich.js"
	ompExtensionsDir = "extensions"

	// ompMCPFile is the document omp reads MCP servers from, in the same agent
	// directory as the extensions. Its contents are mcpDocument's, which Cursor
	// CLI reads the same shape of.
	ompMCPFile = "mcp.json"
)

func (s *Service) ompInstall() error {
	if err := s.installBundle(); err != nil {
		return err
	}
	version, _ := bundleVersion()
	dir, err := ompAgentDir()
	if err != nil {
		return err
	}
	// Merged before anything is written, because the only way this step fails is
	// a document lich must not replace.
	registration, err := mcpDocument(filepath.Join(dir, ompMCPFile), s.lichBin())
	if err != nil {
		return err
	}
	if err := s.installSkills(version, filepath.Join(dir, "skills")); err != nil {
		return err
	}
	if err := removeOmpExtension(); err != nil {
		return err
	}
	if registration == nil {
		return nil
	}
	return writeFile(filepath.Join(dir, ompMCPFile), registration, 0o644)
}

// ompInstalledVersion is the version of the plugin omp sessions load, once the
// install has written what omp reads from its own directory: the theme skill,
// and lich's MCP server unless lich cannot name its binary.
func (s *Service) ompInstalledVersion() (string, bool) {
	dir, err := ompAgentDir()
	if err != nil || !skillPresent(filepath.Join(dir, "skills")) {
		return "", false
	}
	if _, registered := documentCommand(ompMCPPath)(); !registered && s.lichBin() != "" {
		return "", false
	}
	return bundleVersion()
}

// removeOmpExtension deletes the extension lich wrote into omp's extensions
// directory.
func removeOmpExtension() error {
	dir, err := ompAgentDir()
	if err != nil {
		return err
	}
	return removeMarked(filepath.Join(dir, ompExtensionsDir, ompFile))
}

// ompAgentDir is the directory omp keeps its state in — extensions, MCP servers
// and sessions all live under it: Personal's, which internal/ompprofile resolves.
func ompAgentDir() (string, error) { return ompprofile.PersonalDir() }
