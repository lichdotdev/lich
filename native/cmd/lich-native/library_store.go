package main

import (
	"context"
	"log"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// libraryStore is lich's library as the backend last read it, one per window:
// the user's scope loaded at start, a project's once a screen shows its scope,
// and every one of them again on each api.LibraryChangedEvent; read by the
// Skills, MCP servers and Plugins screens and their nav counts.
type libraryStore struct {
	mu     sync.Mutex
	lib    backend.Library
	loaded bool
	// projects are the scopes of the projects asked about, by path; a path
	// asked about and not loaded yet is in asked.
	projects map[string]backend.Library
	asked    map[string]bool
}

// load reads the user's scope and every project's already asked about.
func (s *libraryStore) load(ctx context.Context, client *backend.Client) error {
	lib, err := client.Library(ctx, "")
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.lib, s.loaded = lib, true
	paths := make([]string, 0, len(s.asked))
	for p := range s.asked {
		paths = append(paths, p)
	}
	s.mu.Unlock()
	for _, p := range paths {
		if err := s.loadProject(ctx, client, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *libraryStore) loadProject(ctx context.Context, client *backend.Client, project string) error {
	lib, err := client.Library(ctx, project)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.projects == nil {
		s.projects = map[string]backend.Library{}
	}
	s.projects[project] = lib
	s.mu.Unlock()
	return nil
}

// snapshot is the library in project's scope ("" for the user's), false
// until it was read; a project not asked about yet is read in the background,
// then done runs.
func (s *libraryStore) snapshot(ctx context.Context, client *backend.Client, project string, done func()) (backend.Library, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if project == "" {
		return s.lib, s.loaded
	}
	if lib, ok := s.projects[project]; ok {
		return lib, true
	}
	if !s.asked[project] && client != nil {
		if s.asked == nil {
			s.asked = map[string]bool{}
		}
		s.asked[project] = true
		go func() {
			if err := s.loadProject(ctx, client, project); err != nil {
				log.Printf("library: read %s: %v", project, err)
			}
			done()
		}()
	}
	return backend.Library{}, false
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
	lib := s.lib
	if opts.Project != "" {
		lib = s.projects[opts.Project]
	}
	items := lib.Skills
	switch opts.Kind {
	case api.LibraryMCP:
		items = lib.MCP
	case api.LibraryPluginKind:
		items = lib.Plugins
	}
	for i, it := range items {
		if it.Name != opts.Name {
			continue
		}
		for j, sw := range it.Switches {
			if sw.Provider == opts.Provider && sw.Profile == opts.Profile {
				items[i].Switches[j].On = opts.On
			}
		}
	}
}
