package agentplugin

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
)

// Codex keeps a plugin set per profile, so the lich plugin Personal runs is not
// the one a profile runs. A profile follows Personal: once
// Personal's plugin is installed or updated, every other profile of that
// provider gets the same pinned install, run as the profile, one after the
// other. Claude Code and oh-my-pi need none of this: every session of theirs
// loads lich's own copy of the plugin (bundle.go).
//
// A profile needs no login for it: `codex plugin add` installs into a
// logged-out home (measured on codex-cli 0.162.1), and the install after a
// login runs again anyway.

// profileProvider is what a provider with profiles brings to the pass.
// ensureShared brings a profile to the layout a spawn would before its plugins
// are read: a Codex profile made when config.toml was shared still links
// Personal's, so it would read as holding Personal's plugin, and its next spawn
// rebuilds it without one.
type profileProvider struct {
	list         func() ([]profilestore.Profile, error)
	ensureShared func(slug string) error
	env          func(base []string, dir string) []string
	version      func(s *Service, env []string, dir string) (string, bool)
	install      func(s *Service, env []string) error
}

var profileProviders = map[string]profileProvider{
	providers.Codex: {
		list:         codexprofile.List,
		ensureShared: codexprofile.EnsureShared,
		env:          codexprofile.Env,
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

// followPersonal is one pass over a provider's profiles. A profile already at
// the release is left alone, and so is one that has the plugin while Personal
// itself is behind the release: Personal's own update brings both. The error
// names each profile whose install failed.
func (s *Service) followPersonal(provider string) error {
	kind := profileProviders[provider]
	profiles, err := kind.list()
	if err != nil {
		return fmt.Errorf("list %s profiles: %w", providerName(provider), err)
	}
	profiles = profilesBesidePersonal(profiles)
	if len(profiles) == 0 {
		return nil
	}
	personal, ok := s.installedVersion(provider)
	if !ok {
		return nil
	}
	target, err := s.releaseVersion()
	if err != nil {
		slog.Warn("agentplugin: profiles keep their plugin", "provider", provider, "err", err)
		return nil
	}
	var failed []error
	for _, p := range profiles {
		if _, err := s.follow(kind, p, personal == target, target); err != nil {
			failed = append(failed, fmt.Errorf("profile %s: %w", p.Name, err))
		}
	}
	return errors.Join(failed...)
}

// follow installs the release into one profile when it should have it, and
// reports whether it tried. A profile it could not lay out counts as tried, so
// the failure is reported.
func (s *Service) follow(kind profileProvider, p profilestore.Profile, personalCurrent bool, target string) (bool, error) {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	if err := kind.ensureShared(p.Slug); err != nil {
		return true, err
	}
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
