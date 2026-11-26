package library

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

// watchEvery is how often the providers' own folders and documents are read
// again for what they hold. go.mod carries no file watcher, and a skill or
// server installed by hand is not a thing to show within the second.
const watchEvery = 30 * time.Second

// Watch reports on api.LibraryChangedEvent each time what the providers hold
// beside the library changes, until ctx ends.
func (s *Service) Watch(ctx context.Context) {
	tick := time.NewTicker(watchEvery)
	defer tick.Stop()
	last := s.fingerprint()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if now := s.fingerprint(); now != last {
			last = now
			s.changed()
		}
	}
}

// fingerprint is what the providers hold, their plugins and what the projects
// the screens asked about declare, as one comparable string.
func (s *Service) fingerprint() string {
	s.projectsMu.Lock()
	projects := make([]string, 0, len(s.projects))
	for p := range s.projects {
		projects = append(projects, p)
	}
	s.projectsMu.Unlock()
	sort.Strings(projects)
	state := []any{s.found(), s.plugins("")}
	for _, p := range projects {
		state = append(state, s.projectServers(p), s.plugins(p))
	}
	data, _ := json.Marshal(state)
	return string(data)
}
