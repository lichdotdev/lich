package agentplugin

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/semver"
)

// The Claude Code side: `claude plugin ...` for every mutation, and the CLI's
// own installed_plugins.json for what is installed.

func (s *Service) claudeInstall() error { return s.claudeInstallIn(nil) }

// claudeInstallIn installs the plugin with env as the CLI's environment: nil
// for lich's own, which is Personal, or a profile's (InstallInClaudeProfile).
func (s *Service) claudeInstallIn(env []string) error {
	if err := s.claudePinMarketplace(env); err != nil {
		return err
	}
	return s.runIn(env, providers.Claude, "plugin", "install", pluginKey)
}

// PluginKey is how Claude Code names the lich plugin, the key a port of a
// profile's plugins leaves to InstallInClaudeProfile.
const PluginKey = pluginKey

// InstallInClaudeProfile installs the plugin into one Claude Code profile, run
// as that profile so its account's plugin policy applies. It is the install and
// the update both, as Install is for Personal.
func (s *Service) InstallInClaudeProfile(slug string) error {
	env, err := claudeProfileEnv(slug)
	if err != nil {
		return err
	}
	return s.claudeInstallIn(env)
}

// ClaudeProfileStatus is Status's Claude Code row for one profile: a profile
// has a plugin set of its own.
func (s *Service) ClaudeProfileStatus(slug string) (Status, error) {
	dir, err := claudeprofile.ConfigDir(slug)
	if err != nil {
		return Status{}, err
	}
	available := s.available(providers.Claude)
	version, installed := "", false
	if available {
		version, installed = claudeInstalledVersionIn(dir)
	}
	return computeStatus(providers.Claude, available, installed, version, s.latestVersion()), nil
}

// claudeProfileEnv is the CLI environment for a profile: nil for Personal,
// which runs in lich's own, as its sessions do.
func claudeProfileEnv(slug string) ([]string, error) {
	if claudeprofile.IsPersonal(slug) {
		return nil, nil
	}
	dir, err := claudeprofile.ConfigDir(slug)
	if err != nil {
		return nil, err
	}
	return claudeprofile.Env(os.Environ(), dir), nil
}

// claudePinMarketplace declares the marketplace at the tag of the newest release
// this lich is compatible with, so neither an install nor Claude Code's own
// marketplace refresh can move the plugin past it.
//
// The declaration is removed first because Claude Code refuses an add whose ref
// differs from the one already declared, and has no command to change a ref.
// The remove also uninstalls the plugin, which is why Update is this same path
// and not `plugin update`: the install right after puts it back at the pinned
// release, a downgrade included. A remove with nothing declared fails, which is
// the first install and not an error. All measured on Claude Code 2.1.270.
func (s *Service) claudePinMarketplace(env []string) error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	if err := s.runIn(env, providers.Claude, "plugin", "marketplace", "remove", marketplaceName); err != nil {
		slog.Debug("agentplugin: claude marketplace remove", "err", err)
	}
	return s.runIn(env, providers.Claude, "plugin", "marketplace", "add", gitURL+"#v"+version)
}

// SubagentCardsRelease is the first lich-plugin release whose Claude Code mod
// runs a general-purpose subagent as a lich session (hooks/agent-cards.js).
const SubagentCardsRelease = "0.15.0"

// ClaudeRunsSubagentCards reports whether the installed Claude Code plugin is a
// release whose mod turns subagents into lich sessions. It reads Claude Code's
// plugin state on each call.
func ClaudeRunsSubagentCards() bool {
	version, ok := claudeInstalledVersion()
	return ok && runsSubagentCards(version)
}

func runsSubagentCards(version string) bool {
	return !semver.Less(version, SubagentCardsRelease)
}

// NestedSubagentCardsRelease is the first lich-plugin release whose mod tells a
// worker by LICH_SUBAGENT_DEPTH rather than by LICH_SUBAGENT_CARDS=off
// (hooks/worker-answer.js), so a worker can open cards of its own and still
// answer its errand. An older one reads a worker without that variable as a
// session that answers nothing.
const NestedSubagentCardsRelease = "0.19.0"

// ClaudeNestsSubagentCards reports whether the installed Claude Code plugin is a
// release a worker can open cards from. It reads Claude Code's plugin state on
// each call.
func ClaudeNestsSubagentCards() bool {
	version, ok := claudeInstalledVersion()
	return ok && nestsSubagentCards(version)
}

func nestsSubagentCards(version string) bool {
	return !semver.Less(version, NestedSubagentCardsRelease)
}

// claudeInstalledVersion reads the plugin's installed version from Claude
// Code's plugin state in Personal, or ("", false) when absent or unreadable.
func claudeInstalledVersion() (string, bool) {
	dir := claudeConfigDir()
	if dir == "" {
		return "", false
	}
	return claudeInstalledVersionIn(dir)
}

// ClaudeInstalledIn reports whether the plugin is installed in the Claude Code
// config directory dir, a profile's own plugin state.
func ClaudeInstalledIn(dir string) bool {
	_, ok := claudeInstalledVersionIn(dir)
	return ok
}

func claudeInstalledVersionIn(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "plugins", "installed_plugins.json"))
	if err != nil {
		return "", false
	}
	return parseInstalledVersion(data, pluginKey)
}

// parseInstalledVersion pulls the plugin's version out of installed_plugins.json,
// preferring the user-scope install (how lich installs it) over any other.
func parseInstalledVersion(data []byte, key string) (string, bool) {
	var doc struct {
		Plugins map[string][]struct {
			Scope   string `json:"scope"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	entries := doc.Plugins[key]
	for _, e := range entries {
		if e.Scope == "user" && e.Version != "" {
			return e.Version, true
		}
	}
	for _, e := range entries {
		if e.Version != "" {
			return e.Version, true
		}
	}
	return "", false
}

// claudeConfigDir resolves Claude Code's config directory: the CLAUDE_CONFIG_DIR
// override, else ~/.claude.
func claudeConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}
