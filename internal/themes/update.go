package themes

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/semver"
)

// updateCheckEvery is how often the installed packs are cloned again to look
// for a newer version: a pack is released rarely, and each check is one
// shallow clone per repository.
const updateCheckEvery = 6 * time.Hour

// WatchUpdates checks the installed packs for a newer version now and every
// updateCheckEvery, until ctx ends. It only looks: installing is the user's.
func (s *Service) WatchUpdates(ctx context.Context) {
	tick := time.NewTicker(updateCheckEvery)
	defer tick.Stop()
	for {
		if _, err := s.CheckUpdates(ctx, api.None{}); err != nil {
			slog.Warn("themes: check for updates", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// CheckUpdates clones every repository an installed theme came from, once per
// repository, and remembers the version its manifest carries; List then marks
// the themes installed at an older one. A repository that cannot be read keeps
// what the last check found for it and is named in the error.
func (s *Service) CheckUpdates(_ context.Context, _ api.None) (api.None, error) {
	if s.initErr != nil {
		return api.None{}, s.initErr
	}
	s.checking.Lock()
	defer s.checking.Unlock()
	custom, _, err := s.customThemes()
	if err != nil {
		return api.None{}, err
	}
	s.mu.Lock()
	known := s.latest
	s.mu.Unlock()
	found := make(map[string]api.ThemeLatest, len(known))
	seen := map[string]struct{}{}
	var failed []error
	for _, theme := range custom {
		if theme.Source == nil {
			continue
		}
		url := theme.Source.URL
		if _, done := seen[url]; done {
			continue
		}
		seen[url] = struct{}{}
		remote, err := fetchPack(url, s.lichVersion)
		if err != nil {
			failed = append(failed, err)
			if last, ok := known[url]; ok {
				found[url] = last
			}
			continue
		}
		found[url] = api.ThemeLatest{Pack: remote.manifest.Name, Version: remote.manifest.version()}
	}
	s.mu.Lock()
	s.latest = found
	s.mu.Unlock()
	if !maps.Equal(known, found) {
		events.Send(s.emit, api.ThemeUpdatesEvent, api.None{})
	}
	return api.None{}, errors.Join(failed...)
}

// markLatest sets Latest on the themes whose pack the last check found at a
// newer version than the one they were installed at.
func (s *Service) markLatest(themes []Theme) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, theme := range themes {
		if theme.Source == nil {
			continue
		}
		if latest, ok := s.latest[theme.Source.URL]; ok && semver.Less(theme.Source.Version, latest.Version) {
			themes[i].Latest = &latest
		}
	}
}
