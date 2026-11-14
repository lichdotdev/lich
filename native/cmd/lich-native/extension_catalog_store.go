package main

import (
	"context"
	"log"
	"sync"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// catalogStore is the lich catalog as the backend holds it, read with
// extensions.Catalog the first time a screen asks and kept by
// extension-catalog, and each image it names, read through the backend once
// per URL.
type catalogStore struct {
	client     *backend.Client
	invalidate func()

	mu      sync.Mutex
	cat     *backend.ExtensionCatalog
	err     string
	reading bool
	// images is each URL's picture, nil while it loads or once it failed.
	images map[string]*ui.ImageData
	asked  map[string]bool
}

func newCatalogStore(client *backend.Client, invalidate func()) *catalogStore {
	return &catalogStore{client: client, invalidate: invalidate, images: map[string]*ui.ImageData{}, asked: map[string]bool{}}
}

// read is the catalog, or why there is none: both empty while the read runs.
// The first call starts it.
func (s *catalogStore) read() (*backend.ExtensionCatalog, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil && s.err == "" && !s.reading {
		s.fetch()
	}
	return s.cat, s.err
}

// retry is Try again: one more read while there is no catalog.
func (s *catalogStore) retry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat == nil && !s.reading {
		s.err = ""
		s.fetch()
	}
}

// fetch reads the catalog. A copy extension-catalog brought while it ran is
// as new as the answer or newer, and wins. Called with mu held.
func (s *catalogStore) fetch() {
	s.reading = true
	go func() {
		cat, err := s.client.ExtensionCatalog(context.Background())
		s.mu.Lock()
		s.reading = false
		switch {
		case s.cat != nil:
		case err != nil:
			s.err = backend.ErrorText(err)
		default:
			s.cat = &cat
		}
		s.mu.Unlock()
		s.invalidate()
	}()
}

// set takes the copy extension-catalog carries.
func (s *catalogStore) set(cat backend.ExtensionCatalog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cat, s.err = &cat, ""
}

// image is the picture at url: nil for none, while it loads, or when it
// failed, which leaves the glyph in its place for good.
func (s *catalogStore) image(url string) *ui.ImageData {
	if url == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asked[url] {
		return s.images[url]
	}
	s.asked[url] = true
	go func() {
		img, err := s.readImage(url)
		if err != nil {
			log.Printf("catalog image %s: %v", url, err)
			return
		}
		s.mu.Lock()
		s.images[url] = img
		s.mu.Unlock()
		s.invalidate()
	}()
	return nil
}

func (s *catalogStore) readImage(url string) (*ui.ImageData, error) {
	data, err := s.client.ExtensionCatalogImage(context.Background(), url)
	if err != nil {
		return nil, err
	}
	return ui.DecodeImage(data)
}
