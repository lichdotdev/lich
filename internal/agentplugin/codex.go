package agentplugin

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/omartelo/lich/internal/codexprofile"
	"github.com/omartelo/lich/internal/providers"
)

// The Codex side: `codex plugin ...` for every mutation, and `codex plugin list
// --json` for what is installed. Codex has no `plugin update`, so an update is
// the same pinned install.

// codexInstallIn installs the plugin with env as the CLI's environment: nil for
// lich's own, which is Personal, or a profile's (InstallInCodexProfile).
func (s *Service) codexInstallIn(env []string) error {
	if err := s.codexPinMarketplace(env); err != nil {
		return err
	}
	return s.runIn(env, providers.Codex, "plugin", "add", pluginKey)
}

// InstallInCodexProfile installs the plugin into one Codex profile, run as that
// profile: a profile has a plugin set and downloads of its own, so the pinned
// reinstall there never touches Personal's. It is the install and the update
// both, as Install is for Personal.
func (s *Service) InstallInCodexProfile(slug string) error {
	var env []string
	if !codexprofile.IsPersonal(slug) {
		dir, err := codexprofile.ConfigDir(slug)
		if err != nil {
			return err
		}
		env = codexprofile.Env(os.Environ(), dir)
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	return s.codexInstallIn(env)
}

// codexPinMarketplace declares the marketplace at the tag of the newest release
// this lich is compatible with, for the reason claudePinMarketplace does. Codex
// refuses an add from a different source ("remove it before adding this
// source"), so the old declaration goes first; a remove with nothing declared
// fails, which is the first install. `plugin add` then installs from the fresh
// clone at that tag. Measured on codex-cli 0.153.4.
func (s *Service) codexPinMarketplace(env []string) error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	if err := s.runIn(env, providers.Codex, "plugin", "marketplace", "remove", marketplaceName); err != nil {
		slog.Debug("agentplugin: codex marketplace remove", "err", err)
	}
	return s.runIn(env, providers.Codex, "plugin", "marketplace", "add", marketplaceRepo, "--ref", "v"+version)
}

// codexInstalledVersion reads the plugin's installed version from Codex's own
// plugin listing, or ("", false) when the plugin is absent or the call fails.
func (s *Service) codexInstalledVersion() (string, bool) {
	out, err := s.read(providers.Codex, "plugin", "list", "--json")
	if err != nil {
		return "", false
	}
	return parseCodexPluginList([]byte(out), pluginKey)
}

// parseCodexPluginList pulls the plugin's version out of `codex plugin list
// --json`. The command prints a banner on some setups, so the JSON object is
// taken from the first brace on — anything before it is not part of the
// document.
func parseCodexPluginList(data []byte, key string) (string, bool) {
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	var doc struct {
		Installed []struct {
			PluginID  string `json:"pluginId"`
			Version   string `json:"version"`
			Installed bool   `json:"installed"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(data[start:], &doc); err != nil {
		return "", false
	}
	for _, p := range doc.Installed {
		if p.PluginID == key && p.Installed && p.Version != "" {
			return p.Version, true
		}
	}
	return "", false
}
