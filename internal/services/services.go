// Package services runs a project's services: commands the user saved once
// (a container, a tunnel, a dev server) that lich starts on a PTY of their own
// and shows as up or down, outside the session list. A service lives as long
// as its project's tab: closing the tab stops it, reopening the project starts
// the ones saved to start with lich, and so does lich starting with the tab
// open. Closing the window leaves them all running; quitting lich stops them
// (Shutdown).
//
// A service is not a shell. When its command exits, the service has stopped:
// nothing is respawned and no prompt is left behind, and what it printed last
// stays readable (Output) until it starts again.
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/terminal"
)

// Store is where services are saved, and the projects they run in.
type Store interface {
	Services() ([]api.ServiceConfig, error)
	SaveService(c api.ServiceConfig) error
	DeleteService(id string) error
	ProjectName(projectID string) string
	ProjectPath(projectID string) string
	ProjectOpen(projectID string) bool
}

// terminalPrefix keeps a service's PTY id apart from every session id, which
// are bare hex or UUIDs.
const terminalPrefix = "service-"

// serviceIDBytes is the length of a service id before hex encoding; the same
// as a session id's (internal/spawn), and opaque the same way.
const serviceIDBytes = 16

// Service is the services of every project and the process of each.
type Service struct {
	store Store
	term  *terminal.Service
	emit  func(name string, data any)
	now   func() time.Time

	mu   sync.Mutex
	runs map[string]*run
	// changed is closed and replaced on every change of a run's state, which is
	// what Shutdown waits on.
	changed chan struct{}
	// emitMu orders the ServicesEvent payloads: each is read after the change
	// it reports and sent before the next one is read, so the window never
	// ends on an older list than the last change.
	emitMu sync.Mutex
}

// run is where one service's process is. gen counts its runs, so the exit of
// a run that was replaced cannot overwrite its successor.
type run struct {
	gen       int
	state     string
	code      *int
	startedAt time.Time
	stoppedAt time.Time
	// stopAsked is set when the exit coming is one the user asked for (Stop,
	// ForceStop, or Ctrl+C at the console), which makes it a stop rather than
	// a failure whatever its exit code. restart starts it again once it exits.
	stopAsked bool
	restart   bool
	tail      []byte
}

// New returns the services of store, none of them running.
func New(store Store, term *terminal.Service, emit func(name string, data any)) *Service {
	return &Service{store: store, term: term, emit: emit, now: time.Now, runs: map[string]*run{}, changed: make(chan struct{})}
}

// list is every service with its state, each project's in the order they were
// added.
func (s *Service) list() ([]api.Service, error) {
	configs, err := s.store.Services()
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]api.Service, 0, len(configs))
	for _, c := range configs {
		name, ok := names[c.ProjectID]
		if !ok {
			name = s.store.ProjectName(c.ProjectID)
			names[c.ProjectID] = name
		}
		out = append(out, s.view(c, name))
	}
	return out, nil
}

func (s *Service) view(c api.ServiceConfig, projectName string) api.Service {
	v := api.Service{ServiceConfig: c, ProjectName: projectName, Terminal: terminalPrefix + c.ID, State: api.ServiceStopped}
	r, ok := s.runs[c.ID]
	if !ok {
		return v
	}
	v.State = r.state
	v.ExitCode = r.code
	v.StartedAt = unixMilli(r.startedAt)
	v.StoppedAt = unixMilli(r.stoppedAt)
	return v
}

func unixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// Save writes a service, a new one when opts.ID is empty. The name and the
// command are trimmed and both are required, and the name must be free in its
// project: the footer and the tray tell services apart by it. A running
// service keeps the command it was started with until its next start.
func (s *Service) Save(_ context.Context, opts api.SaveServiceOptions) (api.Service, error) {
	c := api.ServiceConfig{
		ID: opts.ID, ProjectID: opts.ProjectID, Name: strings.TrimSpace(opts.Name),
		Command: strings.TrimSpace(opts.Command), StartWithLich: opts.StartWithLich,
	}
	if err := s.validate(c); err != nil {
		return api.Service{}, err
	}
	if c.ID == "" {
		id, err := newID()
		if err != nil {
			return api.Service{}, err
		}
		c.ID = id
	}
	if err := s.store.SaveService(c); err != nil {
		return api.Service{}, err
	}
	s.publish()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view(c, s.store.ProjectName(c.ProjectID)), nil
}

func (s *Service) validate(c api.ServiceConfig) error {
	if c.Name == "" {
		return errors.New("a service needs a name")
	}
	if c.Command == "" {
		return fmt.Errorf("service %q needs a command to run", c.Name)
	}
	if s.store.ProjectPath(c.ProjectID) == "" {
		return fmt.Errorf("project %q does not exist", c.ProjectID)
	}
	configs, err := s.store.Services()
	if err != nil {
		return err
	}
	for _, other := range configs {
		if other.ID == c.ID && c.ID != "" && other.ProjectID != c.ProjectID {
			return fmt.Errorf("service %q belongs to another project", c.Name)
		}
		if other.ID != c.ID && other.ProjectID == c.ProjectID && strings.EqualFold(other.Name, c.Name) {
			return fmt.Errorf("this project already has a service named %q", other.Name)
		}
	}
	return nil
}

func newID() (string, error) {
	raw := make([]byte, serviceIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate service id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// Delete removes a service that is not running. One that is running is
// refused rather than stopped on the way out: stopping can take a command a
// while, and deleting it under its feet would leave a process nothing lists.
func (s *Service) Delete(_ context.Context, opts api.ServiceIDOptions) (api.None, error) {
	return api.None{}, s.delete(opts.ID)
}

func (s *Service) delete(id string) error {
	c, err := s.config(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	r := s.runs[id]
	busy := r != nil && (r.state == api.ServiceRunning || r.state == api.ServiceStopping)
	s.mu.Unlock()
	if busy {
		return fmt.Errorf("stop %q before deleting it", c.Name)
	}
	if err := s.store.DeleteService(id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.runs, id)
	s.mu.Unlock()
	s.publish()
	return nil
}

func (s *Service) config(id string) (api.ServiceConfig, error) {
	configs, err := s.store.Services()
	if err != nil {
		return api.ServiceConfig{}, err
	}
	for _, c := range configs {
		if c.ID == id {
			return c, nil
		}
	}
	return api.ServiceConfig{}, fmt.Errorf("no service %q", id)
}

// publish tells every window the services as they are now.
func (s *Service) publish() {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()
	list, err := s.list()
	if err != nil {
		slog.Warn("services: read for the window", "err", err)
		return
	}
	events.Send(s.emit, api.ServicesEvent, list)
}

// Reveal asks the window to open service id's console.
func (s *Service) Reveal(id string) {
	events.Send(s.emit, api.ServiceRevealEvent, api.ServiceIDOptions{ID: id})
}
