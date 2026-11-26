package terminal

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/agentplugin"
	"github.com/lichdotdev/lich/internal/providers"
)

// How a session is handed lich's own copy of the plugin (agentplugin.BundleDir),
// rather than depending on one installed in the provider's config. Claude Code
// and Cursor CLI load it as a plugin directory, and a Claude Code plugin of the
// same name installed in its config does not load beside it (measured on
// 2.1.332); oh-my-pi loads its extension with -e and opencode its module from
// the inline config, each beside what its own directory holds (measured on
// 19.1.1 and 1.19.4). Codex, Antigravity, Crush and Kiro CLI take none of these
// and run the plugin installed in their config.
const (
	pluginDirFlag     = "--plugin-dir="
	ompExtensionFlag  = "-e"
	opencodePluginKey = "plugin"
)

// pluginArgs returns the arguments that load the plugin in dir, or nil when lich
// has none or the provider takes it otherwise. Claude Code's --plugin-dir takes
// a list, so the value rides in the same token: alone, it would swallow the
// next plain word.
func pluginArgs(kind, dir string) []string {
	if dir == "" {
		return nil
	}
	switch kind {
	case providers.Claude, providers.Cursor:
		return []string{pluginDirFlag + dir}
	case providers.OMP:
		return []string{ompExtensionFlag, filepath.Join(dir, filepath.FromSlash(agentplugin.OMPExtension))}
	}
	return nil
}

// pluginEnv adds the plugin's module to an opencode session's inline config,
// extending the one the person exports (extendInlineConfig). One lich cannot
// extend is left as it is and the session goes without the plugin, as it goes
// without lich's MCP server (mcpEnv).
func pluginEnv(env []string, kind, dir string) []string {
	if dir == "" || kind != providers.OpenCode {
		return env
	}
	module := fileURL(filepath.Join(dir, filepath.FromSlash(agentplugin.OpencodeModule)))
	extended, err := extendInlineConfig(env, opencodeConfigContentVar, func(config map[string]any) error {
		plugins, _ := config[opencodePluginKey].([]any)
		config[opencodePluginKey] = append(plugins, module)
		return nil
	})
	if err != nil {
		slog.Warn("plugin skipped", "provider", kind, "err", err)
		return env
	}
	return extended
}

// fileURL is path as the file:// URL opencode's plugin list takes, a Windows
// drive included (file:///C:/...).
func fileURL(path string) string {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}

// ensurePluginFor asks for the plugin to be set up when kind's spawn finds it
// missing; it never holds the spawn, which starts without it meanwhile.
func (s *Service) ensurePluginFor(kind string) {
	if s.ensurePlugin != nil {
		s.ensurePlugin(kind)
	}
}

// reenablePlugin turns lich's plugin back on for a Codex or Antigravity spawn
// when the person turned it off (agentplugin.Reenable). A failure costs the
// session its hooks, never its spawn.
func (s *Service) reenablePlugin(kind, projectID, profileDir string) {
	changed, err := agentplugin.Reenable(kind, profileDir, s.store.ProviderBin(kind, projectID))
	if err != nil {
		slog.Warn("turn the lich plugin back on", "provider", kind, "err", err)
		return
	}
	if changed {
		slog.Info("turned the lich plugin back on", "provider", kind)
	}
}
