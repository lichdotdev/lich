package main

import (
	"context"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// servicesStore is every project service and its state, as the backend last
// announced them (api.ServicesEvent): the footer, the rail, the console and
// Settings read it, and only the event writes it.
type servicesStore struct {
	mu   sync.Mutex
	list []backend.Service
}

func (s *servicesStore) set(list []backend.Service) {
	s.mu.Lock()
	s.list = list
	s.mu.Unlock()
}

func (s *servicesStore) load(ctx context.Context, client *backend.Client) error {
	list, err := client.Services(ctx)
	if err != nil {
		return err
	}
	s.set(list)
	return nil
}

// of is projectID's services, in the order they were added.
func (s *servicesStore) of(projectID string) []backend.Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []backend.Service
	for _, v := range s.list {
		if v.ProjectID == projectID {
			out = append(out, v)
		}
	}
	return out
}

func (s *servicesStore) byID(id string) (backend.Service, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.list {
		if v.ID == id {
			return v, true
		}
	}
	return backend.Service{}, false
}

// running is how many services of every project have a process up.
func (s *servicesStore) running() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, v := range s.list {
		if serviceUp(v) {
			n++
		}
	}
	return n
}

// serviceUp is whether a service's process is there: running, or on its way
// out after a stop.
func serviceUp(v backend.Service) bool {
	return v.State == api.ServiceRunning || v.State == api.ServiceStopping
}

// servicesTally is what the footer's closed row says of a project's services:
// how many are up, how many there are, and whether one of them failed.
type servicesTally struct {
	up, total int
	failed    bool
}

func tallyServices(list []backend.Service) servicesTally {
	t := servicesTally{total: len(list)}
	for _, v := range list {
		if serviceUp(v) {
			t.up++
		}
		if v.State == api.ServiceFailed {
			t.failed = true
		}
	}
	return t
}
