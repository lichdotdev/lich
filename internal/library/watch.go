package library

import (
	"context"
	"encoding/json"
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

// fingerprint is what the providers hold, as one comparable string.
func (s *Service) fingerprint() string {
	data, _ := json.Marshal(s.found())
	return string(data)
}
