package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
)

// binVersions is what one binary reports and every binary $PATH holds for its
// provider. Both are empty until the backend answered, and stay empty for a
// CLI that prints no version.
type binVersions struct {
	version string
	onPath  []lichclient.ProviderBinary
}

// versionsOf is what asked reports and what else $PATH holds for kind, ok
// false until the backend answered. Each probe boots the whole CLI, so it is
// asked once per visit and off the frame.
func (p *providerSettings) versionsOf(s *settingsScreen, kind, asked string) (binVersions, bool) {
	key := kind + "\x00" + asked
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.versions[key]; ok {
		return v, true
	}
	if !p.askedVersion[key] {
		if p.askedVersion == nil {
			p.askedVersion = map[string]bool{}
		}
		p.askedVersion[key] = true
		go func() {
			var v binVersions
			var err error
			if v.version, err = s.client.ProviderVersion(s.ctx, asked); err != nil {
				log.Printf("settings: %s version: %v", asked, err)
			}
			if v.onPath, err = s.client.ProviderBinaries(s.ctx, kind); err != nil {
				log.Printf("settings: %s binaries on PATH: %v", kind, err)
			}
			p.mu.Lock()
			if p.versions == nil {
				p.versions = map[string]binVersions{}
			}
			p.versions[key] = v
			p.mu.Unlock()
			s.invalidate()
		}()
	}
	return binVersions{}, false
}

// others are the binaries on $PATH that are not the one a session spawns and
// report a different version from it: the ones a user who meant one of them
// would be surprised by. A copy of the same version is no surprise, and one
// that printed no version cannot be said to differ.
func (v binVersions) others(spawned string) []lichclient.ProviderBinary {
	var out []lichclient.ProviderBinary
	for _, b := range v.onPath {
		if b.Path == spawned || b.Version == "" || b.Version == v.version {
			continue
		}
		out = append(out, b)
	}
	return out
}

// alsoOnPathLine is the sentence naming others, "" when there are none.
func alsoOnPathLine(others []lichclient.ProviderBinary) string {
	if len(others) == 0 {
		return ""
	}
	named := make([]string, len(others))
	for i, b := range others {
		named[i] = b.Path + " " + b.Version
	}
	return fmt.Sprintf(binAlsoOnPath, strings.Join(named, ", "))
}
