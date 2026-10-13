package agentplugin

import (
	"log/slog"
	"os"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/codexprofile"
	"github.com/omartelo/lich/internal/profilestore"
	"github.com/omartelo/lich/internal/providers"
)

// Claude Code and Codex keep a plugin set per profile, so the lich plugin
// Personal runs is not the one a profile runs. A profile follows Personal: once
// Personal's plugin is installed or updated, every other profile of that
// provider gets the same pinned install, run as the profile, one after the
// other. oh-my-pi needs none of this: its extension lives in the `extensions/`
// directory every profile links to Personal's (internal/ompprofile).
//
// A profile needs no login for it: `claude plugin install` and `codex plugin
// add` both install into a logged-out home (measured on Claude Code 2.1.296 and
// codex-cli 0.162.1), and the install after a login runs again anyway.

// ProfilesEventName is the event a pass over a provider's profiles reports on,
// once it has installed into at least one.
const ProfilesEventName = "lich-plugin-profiles"

// ProfilesEvent is what one pass did: the release it installed and the
// profiles where the install failed, in the CLI's own words. Name is the
// provider's display name.
type ProfilesEvent struct {
	Provider string           `json:"provider"`
	Name     string           `json:"name"`
	Version  string           `json:"version"`
	Failures []ProfileFailure `json:"failures"`
}

// ProfileFailure is one profile the plugin could not be installed into.
type ProfileFailure struct {
	Profile string `json:"profile"`
	Name    string `json:"name"`
	Error   string `json:"error"`
}

// profileProvider is what a provider with profiles brings to the pass.
type profileProvider struct {
	list    func() ([]profilestore.Profile, error)
	env     func(base []string, dir string) []string
	version func(s *Service, env []string, dir string) (string, bool)
	install func(s *Service, env []string) error
}

var profileProviders = map[string]profileProvider{
	providers.Claude: {
		list: claudeprofile.List,
		env:  claudeprofile.Env,
		version: func(_ *Service, _ []string, dir string) (string, bool) {
			return claudeInstalledVersionIn(dir)
		},
		install: (*Service).claudeInstallIn,
	},
	providers.Codex: {
		list: codexprofile.List,
		env:  codexprofile.Env,
		version: func(s *Service, env []string, _ string) (string, bool) {
			out, err := s.execIn(env, providers.Codex, readTimeout, "plugin", "list", "--json")
			if err != nil {
				return "", false
			}
			return parseCodexPluginList([]byte(out), pluginKey)
		},
		install: (*Service).codexInstallIn,
	},
}

// SetEmit wires where a pass over the profiles reports (ProfilesEventName).
// Startup wiring; without it Install reaches Personal alone.
func (s *Service) SetEmit(emit func(name string, data any)) { s.emit = emit }

// SyncProfiles brings every profile of every provider that has them to the
// release Personal's plugin is at, installing it where it is missing: a
// profile logged in before it got the plugin at login has none. It runs at
// backend start, off the request path, and reports like an update does.
func (s *Service) SyncProfiles() {
	for provider := range profileProviders {
		s.followPersonal(provider)
	}
}

// followPersonal is one pass over a provider's profiles. A profile already at
// the release is left alone, and so is one that has the plugin while Personal
// itself is behind the release: Personal's own update brings both.
func (s *Service) followPersonal(provider string) {
	kind := profileProviders[provider]
	profiles, err := kind.list()
	if err != nil {
		slog.Warn("agentplugin: list profiles", "provider", provider, "err", err)
		return
	}
	profiles = profilesBesidePersonal(profiles)
	if len(profiles) == 0 {
		return
	}
	personal, ok := s.installedVersion(provider)
	if !ok {
		return
	}
	target, err := s.releaseVersion()
	if err != nil {
		slog.Warn("agentplugin: profiles keep their plugin", "provider", provider, "err", err)
		return
	}
	ev := ProfilesEvent{Provider: provider, Name: providerName(provider), Version: target, Failures: []ProfileFailure{}}
	installed := 0
	for _, p := range profiles {
		tried, err := s.follow(kind, p, personal == target, target)
		if tried {
			installed++
		}
		if err != nil {
			ev.Failures = append(ev.Failures, ProfileFailure{Profile: p.Slug, Name: p.Name, Error: err.Error()})
		}
	}
	if installed > 0 && s.emit != nil {
		s.emit(ProfilesEventName, ev)
	}
}

// follow installs the release into one profile when it should have it, and
// reports whether it tried.
func (s *Service) follow(kind profileProvider, p profilestore.Profile, personalCurrent bool, target string) (bool, error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	env := kind.env(os.Environ(), p.Dir)
	version, installed := kind.version(s, env, p.Dir)
	if version == target || (installed && !personalCurrent) {
		return false, nil
	}
	return true, kind.install(s, env)
}

func profilesBesidePersonal(all []profilestore.Profile) []profilestore.Profile {
	var out []profilestore.Profile
	for _, p := range all {
		if !p.Personal {
			out = append(out, p)
		}
	}
	return out
}
