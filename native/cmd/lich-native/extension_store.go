package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"log"
	"maps"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui/icons"
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
	// images is each surface's own icon, decoded once per status.
	images map[extensionKey]*icons.Bitmap
	// footers is each placed surface's footer text by checkout, kept by
	// extension-message until the extension fails or goes.
	footers map[extensionKey]map[string]string
	// paths is every checkout the extensions were told was active: a footer
	// text for another is dropped, which bounds what one can make the window
	// hold.
	paths map[string]bool
}

func (s *extensionStore) load(ctx context.Context, client *backend.Client) error {
	all, err := client.Extensions(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = all
	for _, e := range all {
		s.setImages(e)
	}
	return nil
}

// setStatus takes one extension's new state, in its place in the list, or
// last for one the list never had.
func (s *extensionStore) setStatus(e backend.Extension) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setImages(e)
	if e.Status == api.ExtensionFailed || e.Status == api.ExtensionInvalid {
		s.dropFooters(e.ID)
	}
	if i := slices.IndexFunc(s.list, func(x backend.Extension) bool { return x.ID == e.ID }); i >= 0 {
		s.list[i] = e
		return
	}
	s.list = append(s.list, e)
}

// setLatest records the newer release of extension id, "" for none.
func (s *extensionStore) setLatest(id, latest string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := slices.IndexFunc(s.list, func(x backend.Extension) bool { return x.ID == id }); i >= 0 {
		s.list[i].Latest = latest
	}
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
	s.dropFooters(id)
	s.setImages(backend.Extension{ID: id})
}

// dropFooters forgets extension id's footer texts. Called with mu held.
func (s *extensionStore) dropFooters(id string) {
	maps.DeleteFunc(s.footers, func(k extensionKey, _ map[string]string) bool { return k.ext == id })
}

// sawPath records a checkout the extensions were told was active.
func (s *extensionStore) sawPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paths == nil {
		s.paths = map[string]bool{}
	}
	s.paths[path] = true
}

// setFooter keeps a footer text of extension id, for every checkout or for
// one it was told was active; an empty text clears it.
func (s *extensionStore) setFooter(id string, f extension.FooterData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.Path != "" && !s.paths[f.Path] {
		return
	}
	k := extensionKey{id, f.Surface}
	if f.Text == "" {
		delete(s.footers[k], f.Path)
		return
	}
	if s.footers == nil {
		s.footers = map[extensionKey]map[string]string{}
	}
	if s.footers[k] == nil {
		s.footers[k] = map[string]string{}
	}
	s.footers[k][f.Path] = f.Text
}

// footer is surface k's text for the checkout at path, else its text for
// every checkout.
func (s *extensionStore) footer(k extensionKey, path string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if text, ok := s.footers[k][path]; ok {
		return text
	}
	return s.footers[k][""]
}

// hasPlaced is whether extension id has a surface in the dock or a panel.
func (s *extensionStore) hasPlaced(id string) bool {
	e, ok := s.byID(id)
	return ok && hasPlaced(e)
}

func hasPlaced(e backend.Extension) bool {
	return slices.ContainsFunc(e.Surfaces, func(s backend.ExtensionSurface) bool { return s.Place != string(extension.PlaceScreen) })
}

// setImages replaces e's surface images with the icons it sends. One that
// does not decode is left out, and its surface draws the icon it names.
// Called with mu held.
func (s *extensionStore) setImages(e backend.Extension) {
	maps.DeleteFunc(s.images, func(k extensionKey, _ *icons.Bitmap) bool { return k.ext == e.ID })
	for surface, data := range e.Icons {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			log.Printf("extension %s: icon of %s: %v", e.ID, surface, err)
			continue
		}
		if s.images == nil {
			s.images = map[extensionKey]*icons.Bitmap{}
		}
		s.images[extensionKey{e.ID, surface}] = icons.Picture(img)
	}
}

func (s *extensionStore) surfaceImages() map[extensionKey]*icons.Bitmap {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.images)
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

// extensionEvent applies extension-status, extension-update,
// extension-removed and extension-message. A render and a footer text land in
// the store; every other message is the window's to act on.
func (m *model) extensionEvent(ev backend.Event) {
	switch ev.Name {
	case api.ExtensionStatusEvent.Name:
		if e, err := backend.Decode(ev, api.ExtensionStatusEvent); err == nil {
			was, _ := m.extensions.byID(e.ID)
			m.extensions.setStatus(e)
			if e.Status == api.ExtensionRunning && was.Status != api.ExtensionRunning && m.extensionStarted != nil {
				m.extensionStarted(e.ID)
			}
		}
		return
	case api.ExtensionUpdateEvent.Name:
		if e, err := backend.Decode(ev, api.ExtensionUpdateEvent); err == nil {
			m.extensions.setLatest(e.ID, e.Latest)
			if m.extensionUpdate != nil {
				m.extensionUpdate(e.ID)
			}
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
	if f, ok := msg.Data.(extension.FooterData); ok {
		m.extensions.setFooter(in.ExtensionID, f)
		return
	}
	if m.extensionMessage != nil {
		m.extensionMessage(in.ExtensionID, msg)
	}
}
