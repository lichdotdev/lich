package main

import (
	"context"
	"encoding/json"
	"log"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
)

// extensionKey names one surface of one extension.
type extensionKey struct{ ext, surface string }

// extensionStore is every extension as the backend last announced it, read
// once with extensions.List and kept by extension-status, and each surface's
// last render, kept by extension-message. A render outlives a restart: the
// screen keeps drawing it until the new process sends its own.
type extensionStore struct {
	mu      sync.Mutex
	list    []backend.Extension
	renders map[extensionKey]extension.RenderData
}

func (s *extensionStore) load(ctx context.Context, client *backend.Client) error {
	all, err := client.Extensions(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = all
	return nil
}

// setStatus takes one extension's new state, in its place in the list, or
// last for one the list never had.
func (s *extensionStore) setStatus(e backend.Extension) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := slices.IndexFunc(s.list, func(x backend.Extension) bool { return x.ID == e.ID }); i >= 0 {
		s.list[i] = e
		return
	}
	s.list = append(s.list, e)
}

// remove drops extension id and its surfaces' renders.
func (s *extensionStore) remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = slices.DeleteFunc(s.list, func(x backend.Extension) bool { return x.ID == id })
	for k := range s.renders {
		if k.ext == id {
			delete(s.renders, k)
		}
	}
}

func (s *extensionStore) setRender(id string, r extension.RenderData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.renders == nil {
		s.renders = map[extensionKey]extension.RenderData{}
	}
	s.renders[extensionKey{id, r.Surface}] = r
}

func (s *extensionStore) all() []backend.Extension {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.list)
}

func (s *extensionStore) byID(id string) (backend.Extension, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.list, func(x backend.Extension) bool { return x.ID == id })
	if i < 0 {
		return backend.Extension{}, false
	}
	return s.list[i], true
}

func (s *extensionStore) render(k extensionKey) (extension.RenderData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.renders[k]
	return r, ok
}

// extensionEvent applies extension-status, extension-removed and
// extension-message. A render lands in the store; every other message is the
// window's to act on.
func (m *model) extensionEvent(ev backend.Event) {
	switch ev.Name {
	case api.ExtensionStatusEvent.Name:
		if e, err := backend.Decode(ev, api.ExtensionStatusEvent); err == nil {
			m.extensions.setStatus(e)
		}
		return
	case api.ExtensionRemovedEvent.Name:
		if e, err := backend.Decode(ev, api.ExtensionRemovedEvent); err == nil {
			m.extensions.remove(e.ID)
		}
		return
	}
	in, err := backend.Decode(ev, api.ExtensionMessageEvent)
	if err != nil {
		log.Printf("%s: %v", ev.Name, err)
		return
	}
	var msg extension.Message
	if err := json.Unmarshal(in.Message, &msg); err != nil {
		log.Printf("%s from %s: %v", ev.Name, in.ExtensionID, err)
		return
	}
	if r, ok := msg.Data.(extension.RenderData); ok {
		m.extensions.setRender(in.ExtensionID, r)
		return
	}
	if m.extensionMessage != nil {
		m.extensionMessage(in.ExtensionID, msg)
	}
}
