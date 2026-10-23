package main

import (
	"log"
	"strings"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// binVersions is what one binary reports and every binary $PATH holds for its
// provider. Both are empty until the backend answered, and stay empty for a
// CLI that prints no version.
type binVersions struct {
	version string
	onPath  []backend.ProviderBinary
}

// versionsOf is what asked reports and what else $PATH holds for kind, ok
// false until the backend answered. Each probe boots the whole CLI, so it is
// asked once per visit and off the frame.
func (p *providerSettings) versionsOf(s *settingsScreen, kind, asked string) (binVersions, bool) {
	return p.versions.get(&p.mu, kind+"\x00"+asked, s.invalidate, func() (binVersions, bool) {
		var v binVersions
		var err error
		if v.version, err = s.client.ProviderVersion(s.ctx, asked); err != nil {
			log.Printf("settings: %s version: %v", asked, err)
		}
		if v.onPath, err = s.client.ProviderBinaries(s.ctx, kind); err != nil {
			log.Printf("settings: %s binaries on PATH: %v", kind, err)
		}
		return v, true
	})
}

// others are the binaries on $PATH that are not the one a session spawns and
// report a different version from it: the ones a user who meant one of them
// would be surprised by. A copy of the same version is no surprise, and one
// that printed no version cannot be said to differ.
func (v binVersions) others(spawned string) []backend.ProviderBinary {
	var out []backend.ProviderBinary
	for _, b := range v.onPath {
		if b.Path == spawned || b.Version == "" || b.Version == v.version {
			continue
		}
		out = append(out, b)
	}
	return out
}

// alsoOnPathLine is the sentence naming others, "" when there are none.
func alsoOnPathLine(others []backend.ProviderBinary) string {
	if len(others) == 0 {
		return ""
	}
	named := make([]string, len(others))
	for i, b := range others {
		named[i] = b.Path + " " + b.Version
	}
	return i18n.T("settings.settingsProvidersVersion.alsoOnPath", i18n.Args{"versions": strings.Join(named, ", ")})
}
