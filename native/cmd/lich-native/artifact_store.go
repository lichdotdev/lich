package main

import (
	"context"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
)

// artifactStore is every open session's artifacts, as the backend last
// announced them (api.ArtifactsEvent): the dock's Artifacts tab reads it, and
// only the event and the attach-time read write it. unread is the window's
// own half: a file that arrived or changed after the window attached, until
// the person opens it.
type artifactStore struct {
	mu     sync.Mutex
	lists  map[string][]backend.Artifact
	unread map[string]map[string]bool
}

func (s *artifactStore) load(ctx context.Context, client *backend.Client) error {
	all, err := client.Artifacts(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lists, s.unread = map[string][]backend.Artifact{}, map[string]map[string]bool{}
	for _, a := range all {
		if len(a.Files) > 0 {
			s.lists[a.SessionID] = a.Files
		}
	}
	return nil
}

// apply takes one session's whole list. A file not in its last list, or
// written since, is unread; one still unread stays so.
func (s *artifactStore) apply(a backend.SessionArtifacts) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lists == nil {
		s.lists, s.unread = map[string][]backend.Artifact{}, map[string]map[string]bool{}
	}
	if len(a.Files) == 0 {
		delete(s.lists, a.SessionID)
		delete(s.unread, a.SessionID)
		return
	}
	was := map[string]int64{}
	for _, f := range s.lists[a.SessionID] {
		was[f.Name] = f.Modified
	}
	fresh := map[string]bool{}
	for _, f := range a.Files {
		modified, known := was[f.Name]
		if !known || f.Modified > modified || s.unread[a.SessionID][f.Name] {
			fresh[f.Name] = true
		}
	}
	s.lists[a.SessionID], s.unread[a.SessionID] = a.Files, fresh
}

// of is session id's artifacts, newest first.
func (s *artifactStore) of(id string) []backend.Artifact {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.lists[id])
}

func (s *artifactStore) isUnread(id, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unread[id][name]
}

func (s *artifactStore) markRead(id, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.unread[id], name)
}

// artifactGroup is one other session's artifacts, under its line.
type artifactGroup struct {
	session backend.Session
	files   []backend.Artifact
}

// artifactGroups is what the tab lists for the active session: its own files,
// then those of each other open session of its project that has any, in the
// project's order.
func artifactGroups(active string, peers []backend.Session, of func(string) []backend.Artifact) (own []backend.Artifact, others []artifactGroup) {
	own = of(active)
	for _, p := range peers {
		if p.ID == active {
			continue
		}
		if files := of(p.ID); len(files) > 0 {
			others = append(others, artifactGroup{session: p, files: files})
		}
	}
	return own, others
}

// artifactCount is how many rows artifactGroups lists.
func artifactCount(own []backend.Artifact, others []artifactGroup) int {
	n := len(own)
	for _, g := range others {
		n += len(g.files)
	}
	return n
}

// projectPeers is the open sessions of the project session id is in, id's
// own included.
func (m *model) projectPeers(id string) []backend.Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, _ := m.sessionAt(id)
	if p == nil {
		return nil
	}
	return slices.Clone(p.Sessions)
}
