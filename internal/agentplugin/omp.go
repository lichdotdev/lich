package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/ompprofile"
)

// The oh-my-pi side. omp has no plugin CLI either, and its extensions are
// modules it imports from a directory it scans unprompted — so, as with
// opencode, installing one is writing the file and the file being there is the
// install. omp keeps no plugin state to ask, so the marker line lich writes
// above the module is again the only record of what version is installed.
//
// The MCP registration is the part that differs. omp takes no `--mcp-config`
// flag, and the only place it reads a server from is an `mcpServers` document
// beside the extensions — so lich writes one (mcpDocument, shared with Cursor
// CLI, which reads the same shape).

const (
	// ompSource is the module's path inside the plugin repository, and ompFile
	// the name it takes in omp's extensions directory — the directory omp scans
	// unprompted, which is what makes the file being there the whole install.
	ompSource        = "omp/lich.js"
	ompFile          = "lich.js"
	ompExtensionsDir = "extensions"

	// ompMCPFile is the document omp reads MCP servers from, in the same agent
	// directory as the extensions. Its contents are mcpDocument's, which Cursor
	// CLI reads the same shape of.
	ompMCPFile = "mcp.json"
)

func (s *Service) ompInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	data, err := s.fetchFile(version, ompSource)
	if err != nil {
		return err
	}
	dir, err := ompAgentDir()
	if err != nil {
		return err
	}
	// Merged before anything is written, because the only way this step fails is
	// a document lich must not replace — and an install that leaves an extension
	// behind while refusing the registration is worse than one the user retries.
	registration, err := mcpDocument(filepath.Join(dir, ompMCPFile), s.lichBin())
	if err != nil {
		return err
	}
	extensions := filepath.Join(dir, ompExtensionsDir)
	if err := os.MkdirAll(extensions, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", extensions, err)
	}
	body := fmt.Sprintf("%s %s v%s — installed by lich, replace with the file from %s\n%s",
		jsComment, markerName, version, marketplaceRepo, data)
	if err := writeFile(filepath.Join(extensions, ompFile), []byte(body), 0o644); err != nil {
		return err
	}
	if registration == nil {
		return nil
	}
	return writeFile(filepath.Join(dir, ompMCPFile), registration, 0o644)
}

// ompInstalledVersion reads the version off the marker line lich wrote, or
// ("", false) when the module is absent, unreadable, or was not written by lich.
func (s *Service) ompInstalledVersion() (string, bool) {
	dir, err := ompAgentDir()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, ompExtensionsDir, ompFile))
	if err != nil {
		return "", false
	}
	return versionOf(data, jsComment)
}

// ompAgentDir is the directory omp keeps its state in — extensions, MCP servers
// and sessions all live under it: Personal's, which internal/ompprofile resolves.
func ompAgentDir() (string, error) { return ompprofile.PersonalDir() }
