package agentplugin

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/semver"
)

// The Claude Code side. Its sessions load lich's own copy of the plugin with
// `--plugin-dir` (bundle.go), which Cursor CLI's take too, so nothing is
// installed in Claude Code any more: an install of the past is removed from
// Personal and every profile (migrateClaude), and Claude Code's own CLI is
// called for that alone.

// claudeInstallIn unpacks the plugin and removes the install of the past from
// the Claude Code home env names: nil for Personal, or a profile's.
func (s *Service) claudeInstallIn(env []string, dir string) error {
	if err := s.installBundle(); err != nil {
		return err
	}
	return s.uninstallClaudeIn(env, dir)
}

// PluginKey is how Claude Code names the lich plugin, the key a port of a
// profile's plugins leaves to InstallInClaudeProfile.
const PluginKey = pluginKey

// InstallInClaudeProfile readies the plugin for one Claude Code profile, which
// is the release every session loads, and takes the install of the past out of
// the profile's own plugin set.
func (s *Service) InstallInClaudeProfile(slug string) error {
	env, err := claudeProfileEnv(slug)
	if err != nil {
		return err
	}
	dir, err := claudeprofile.ConfigDir(slug)
	if err != nil {
		return err
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	return s.claudeInstallIn(env, claudeprofile.PluginsDir(slug, dir))
}

// ClaudeProfileStatus is Status's Claude Code row for one profile, which runs
// the same release Personal does.
func (s *Service) ClaudeProfileStatus(slug string) (Status, error) {
	if _, err := claudeprofile.ConfigDir(slug); err != nil {
		return Status{}, err
	}
	available := s.available(providers.Claude)
	version, installed := "", false
	if available {
		version, installed = bundleVersion()
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

// uninstallClaudeIn removes the plugin a lich of the past installed into the
// plugin set in plugins, and its marketplace. Without it Cursor CLI, which
// loads every plugin installed in Claude Code beside its own --plugin-dir,
// would run each hook twice (measured on 2026.11.19). The uninstall keeps the
// release's files in Claude Code's cache, so a session started on them runs
// on until it restarts (measured on 2.1.332).
func (s *Service) uninstallClaudeIn(env []string, plugins string) error {
	data, err := os.ReadFile(filepath.Join(plugins, "installed_plugins.json"))
	if err != nil {
		return nil
	}
	if _, installed := parseInstalledVersion(data, pluginKey); !installed {
		return nil
	}
	if err := s.runIn(env, providers.Claude, "plugin", "uninstall", pluginKey); err != nil {
		return err
	}
	if err := s.runIn(env, providers.Claude, "plugin", "marketplace", "remove", marketplaceName); err != nil {
		slog.Debug("agentplugin: claude marketplace remove", "err", err)
	}
	return nil
}

// migrateClaude takes the install of the past out of Personal and every
// profile. A profile it cannot reach keeps its copy and is logged: its
// sessions load lich's copy over it, as every one does.
func (s *Service) migrateClaude() {
	if !s.available(providers.Claude) {
		return
	}
	profiles, err := claudeprofile.List()
	if err != nil {
		slog.Warn("agentplugin: list Claude Code profiles", "err", err)
		return
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	for _, p := range profiles {
		env, err := claudeProfileEnv(p.Slug)
		if err == nil {
			err = s.uninstallClaudeIn(env, claudeprofile.PluginsDir(p.Slug, p.Dir))
		}
		if err != nil {
			slog.Warn("agentplugin: remove the Claude Code install", "profile", p.Slug, "err", err)
		}
	}
}

// SubagentCardsRelease is the first lich-plugin release whose Claude Code mod
// runs a general-purpose subagent as a lich session (hooks/agent-cards.js).
const SubagentCardsRelease = "0.15.0"

// ClaudeRunsSubagentCards reports whether the release Claude Code sessions load,
// in any profile (profileDir, "" for Personal), is one whose mod turns
// subagents into lich sessions. It reads the disk on each call.
func ClaudeRunsSubagentCards(profileDir string) bool {
	version, ok := claudeInstalledVersionFor(profileDir)
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

// ClaudeNestsSubagentCards reports whether the release Claude Code sessions
// load, in any profile, is one a worker can open cards from. It reads the disk
// on each call.
func ClaudeNestsSubagentCards(profileDir string) bool {
	version, ok := claudeInstalledVersionFor(profileDir)
	return ok && nestsSubagentCards(version)
}

func nestsSubagentCards(version string) bool {
	return !semver.Less(version, NestedSubagentCardsRelease)
}

// SubagentHookRelease is the first lich-plugin release whose pre-tool hook asks
// /subagent (docs/hooks/subagent-cards.md) and whose mod no longer converts
// subagents itself, so LICH_SUBAGENT_CARDS no longer means anything to it.
const SubagentHookRelease = "0.22.0"

// ClaudeAsksSubagentHook reports whether the release Claude Code sessions load,
// in any profile, is one whose hook asks lich. It reads the disk on each call.
func ClaudeAsksSubagentHook(profileDir string) bool {
	version, ok := claudeInstalledVersionFor(profileDir)
	return ok && asksSubagentHook(version)
}

func asksSubagentHook(version string) bool {
	return !semver.Less(version, SubagentHookRelease)
}

// EditNoteRelease is the first lich-plugin release whose touched report names
// the paths it wrote and hands lich's edit-overlap note to the model
// (docs/hooks/session-touched.md).
const EditNoteRelease = "0.22.0"

func notesEdits(version string) bool {
	return !semver.Less(version, EditNoteRelease)
}

// vetoScript is the hook that asks /subagent for the harnesses lich installs by
// files (Antigravity, Kiro CLI, Crush), from SubagentHookRelease on; it sources
// conversation-id.sh, which every one of them already ships.
const vetoScript = "hooks/veto-subagent.sh"

// claudeInstalledVersionFor is the release a Claude Code session loads, which
// is lich's own whatever the profile.
func claudeInstalledVersionFor(string) (string, bool) {
	return bundleVersion()
}

// ClaudeInstalledIn reports whether a Claude Code session started in the
// config directory dir loads the plugin, which it does whenever lich has one.
func ClaudeInstalledIn(string) bool {
	_, ok := bundleVersion()
	return ok
}

// parseInstalledVersion pulls the plugin's version out of installed_plugins.json.
// Only a user-scope install counts: it is how lich installs the plugin, and a
// project or local entry loads only inside its projectPath.
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
	for _, e := range doc.Plugins[key] {
		if e.Scope == "user" && e.Version != "" {
			return e.Version, true
		}
	}
	return "", false
}

// claudeInstall readies the plugin for every Claude Code session and takes the
// install of the past out of Personal and every profile.
func (s *Service) claudeInstall() error {
	if err := s.installBundle(); err != nil {
		return err
	}
	s.migrateClaude()
	return nil
}
