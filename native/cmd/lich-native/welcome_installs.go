package main

import (
	"context"
	"log"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// installStep is a provider's install as the backend last reported it.
type installStep struct {
	running  bool
	progress float64
	// failure is why the last install failed, in the backend's one line.
	failure string
}

// installBook is the window's one store of the provider installs, fed by
// ProviderInstallEvent; the Welcome reads it. Every method is safe from any
// goroutine.
type installBook struct {
	mu    sync.Mutex
	steps map[string]installStep
}

func (b *installBook) of(id string) installStep {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.steps[id]
}

func (b *installBook) set(id string, step installStep) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.steps == nil {
		b.steps = map[string]installStep{}
	}
	b.steps[id] = step
}

// start shows an install running before the backend's first step arrives.
func (b *installBook) start(id string) { b.set(id, installStep{running: true}) }

// take is one step of an install.
func (b *installBook) take(ev api.ProviderInstall) {
	b.set(ev.ID, installStep{running: !ev.Done, progress: ev.Progress, failure: ev.Error})
}

// installEvent takes a step of an install. A finished one reaches the store
// only once the provider book and the provider's profiles were read again,
// so the screen that shows the install never sees it done with the provider
// still missing.
func (m *model) installEvent(ctx context.Context, ev api.ProviderInstall) {
	if !ev.Done || ev.Error != "" {
		m.installs.take(ev)
		m.invalidate()
		return
	}
	go func() {
		if err := m.providers.reload(ctx); err != nil {
			log.Printf("providers after an install: %v", err)
		}
		if book := m.profileBook(ev.ID); book != nil {
			if err := book.reload(ctx); err != nil {
				log.Printf("%s profiles after an install: %v", ev.ID, err)
			}
		}
		if m.providerInstalled != nil {
			m.providerInstalled(ev.ID)
		}
		m.installs.take(ev)
		m.invalidate()
	}()
}

// installProvider starts installing id; a refusal ends the install on the
// backend's sentence at once.
func (m *model) installProvider(ctx context.Context, c *backend.Client, id string) {
	m.installs.start(id)
	m.invalidate()
	go func() {
		if err := c.InstallProvider(ctx, id); err != nil {
			m.installs.take(api.ProviderInstall{ID: id, Done: true, Error: backend.ErrorText(err)})
			m.invalidate()
		}
	}()
}
