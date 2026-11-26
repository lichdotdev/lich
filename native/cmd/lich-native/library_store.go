package main

import (
	"context"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// libraryStore is lich's library as the backend last read it, one per window:
// loaded at start and again on every api.LibraryChangedEvent, read by the
// Skills, MCP servers and Plugins screens and their nav counts.
type libraryStore struct {
	mu     sync.Mutex
	lib    backend.Library
	loaded bool
}

func (s *libraryStore) load(ctx context.Context, client *backend.Client) error {
	lib, err := client.Library(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.lib, s.loaded = lib, true
	s.mu.Unlock()
	return nil
}

func (s *libraryStore) snapshot() (backend.Library, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lib, s.loaded
}

// newCount is how many found items of kind appeared since the last visit.
func (s *libraryStore) newCount(kind string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, f := range s.lib.Found {
		if f.Kind == kind && f.New {
			n++
		}
	}
	return n
}

// setSwitch files a switch the person just pressed, until the backend's
// answer replaces it.
func (s *libraryStore) setSwitch(opts api.LibrarySwitchOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := &s.lib.Skills
	if opts.Kind == api.LibraryMCP {
		items = &s.lib.MCP
	}
	for i, it := range *items {
		if it.Name != opts.Name {
			continue
		}
		for j, sw := range it.Switches {
			if sw.Provider == opts.Provider && sw.Profile == opts.Profile {
				(*items)[i].Switches[j].On = opts.On
			}
		}
	}
}
