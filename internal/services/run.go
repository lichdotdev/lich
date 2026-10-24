package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/terminal"
)

// start runs service id in its project's directory. One already running or
// stopping is left alone. A command that cannot be started at all fails the
// service with the reason on its console, as well as returning it.
func (s *Service) start(id string) error {
	c, err := s.config(id)
	if err != nil {
		return err
	}
	cwd := s.store.ProjectPath(c.ProjectID)
	if cwd == "" {
		return fmt.Errorf("the project of service %q is gone", c.Name)
	}
	s.mu.Lock()
	r, ok := s.runs[id]
	if !ok {
		r = &run{}
		s.runs[id] = r
	}
	if r.state == api.ServiceRunning || r.state == api.ServiceStopping {
		s.mu.Unlock()
		return nil
	}
	r.gen++
	gen := r.gen
	*r = run{gen: gen, state: api.ServiceRunning, startedAt: s.now()}
	s.mu.Unlock()

	err = s.term.StartService(terminal.ServiceCommand{
		ID: terminalPrefix + id, ProjectID: c.ProjectID, Cwd: cwd, Line: c.Command,
		Interrupted: func() { s.stopAsked(id, gen) },
		Exited:      func(code int, tail []byte) { s.exited(id, gen, code, tail) },
	})
	if err != nil {
		s.mu.Lock()
		if r.gen == gen {
			r.state, r.stoppedAt, r.tail = api.ServiceFailed, s.now(), []byte(err.Error()+"\r\n")
		}
		s.signal()
		s.mu.Unlock()
		s.publish()
		return err
	}
	s.mu.Lock()
	s.signal()
	s.mu.Unlock()
	s.publish()
	return nil
}

// stop asks service id to stop the way Ctrl+C at its console does, and leaves
// it stopping until it exits; how long that takes is the command's to decide.
func (s *Service) stop(id string) error {
	return s.halt(id, s.term.InterruptService)
}

// forceStop kills service id without waiting for it to agree: what a command
// that ignores Stop, or takes too long over it, is left with.
func (s *Service) forceStop(id string) error {
	return s.halt(id, func(tid string) error {
		s.term.KillService(tid)
		return nil
	})
}

func (s *Service) halt(id string, end func(terminalID string) error) error {
	s.mu.Lock()
	r := s.runs[id]
	if r == nil || (r.state != api.ServiceRunning && r.state != api.ServiceStopping) {
		s.mu.Unlock()
		return nil
	}
	r.state, r.stopAsked = api.ServiceStopping, true
	s.signal()
	s.mu.Unlock()
	s.publish()
	return end(terminalPrefix + id)
}

// restart stops service id and starts it again once it exited, or starts it
// when it is not running.
func (s *Service) restart(id string) error {
	s.mu.Lock()
	r := s.runs[id]
	running := r != nil && (r.state == api.ServiceRunning || r.state == api.ServiceStopping)
	if running {
		r.restart = true
	}
	s.mu.Unlock()
	if !running {
		return s.start(id)
	}
	return s.stop(id)
}

// startAll starts every service of projectID that is not running.
func (s *Service) startAll(projectID string) error {
	return s.eachOf(projectID, s.start)
}

// stopAll stops every service of projectID that is running.
func (s *Service) stopAll(projectID string) error {
	return s.eachOf(projectID, s.stop)
}

func (s *Service) eachOf(projectID string, fn func(id string) error) error {
	configs, err := s.store.Services()
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range configs {
		if c.ProjectID == projectID {
			errs = append(errs, fn(c.ID))
		}
	}
	return errors.Join(errs...)
}

// output is what service id printed last, kept from the moment its last run
// exited; nil while it runs (its console is live on /ws then) or before it
// ever ran.
func (s *Service) output(id string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runs[id]
	if r == nil || r.state == api.ServiceRunning || r.state == api.ServiceStopping {
		return nil
	}
	return r.tail
}

// Running is every service whose process is up, stopping ones included.
func (s *Service) Running() []api.Service {
	list, err := s.list()
	if err != nil {
		slog.Warn("services: read the running ones", "err", err)
		return nil
	}
	var out []api.Service
	for _, v := range list {
		if v.State == api.ServiceRunning || v.State == api.ServiceStopping {
			out = append(out, v)
		}
	}
	return out
}

func (s *Service) stopAsked(id string, gen int) {
	s.mu.Lock()
	r := s.runs[id]
	if r == nil || r.gen != gen || r.state != api.ServiceRunning {
		s.mu.Unlock()
		return
	}
	r.state, r.stopAsked = api.ServiceStopping, true
	s.signal()
	s.mu.Unlock()
	s.publish()
}

func (s *Service) exited(id string, gen, code int, tail []byte) {
	s.mu.Lock()
	r := s.runs[id]
	if r == nil || r.gen != gen {
		s.mu.Unlock()
		return
	}
	r.stoppedAt, r.tail = s.now(), tail
	if code >= 0 {
		r.code = &code
	}
	r.state = api.ServiceFailed
	if r.stopAsked || code == 0 {
		r.state = api.ServiceStopped
	}
	again := r.restart
	r.restart = false
	s.signal()
	s.mu.Unlock()
	s.publish()
	if again {
		if err := s.start(id); err != nil {
			slog.Warn("services: restart", "service", id, "err", err)
		}
	}
}

// signal wakes whoever waits on a state change. Called with mu held.
func (s *Service) signal() {
	close(s.changed)
	s.changed = make(chan struct{})
}

// Boot starts every service saved to start with lich. One that fails to start
// is logged and shown failed; it does not hold up the others.
func (s *Service) Boot() {
	configs, err := s.store.Services()
	if err != nil {
		slog.Warn("services: read at boot", "err", err)
		return
	}
	for _, c := range configs {
		if !c.StartWithLich {
			continue
		}
		if err := s.start(c.ID); err != nil {
			slog.Warn("services: start with lich", "service", c.Name, "err", err)
		}
	}
}

// StopRunning asks every running service, of every project, to stop.
func (s *Service) StopRunning() {
	for _, v := range s.Running() {
		if err := s.stop(v.ID); err != nil {
			slog.Warn("services: stop", "service", v.Name, "err", err)
		}
	}
}

// Shutdown stops every running service on lich's way out: asked first, then
// killed once grace is up, and waited for at most a few seconds more so no
// process outlives the backend that started it.
func (s *Service) Shutdown(grace time.Duration) {
	s.StopRunning()
	if s.waitIdle(grace) {
		return
	}
	for _, v := range s.Running() {
		slog.Warn("services: killed on quit", "service", v.Name, "grace", grace)
		_ = s.forceStop(v.ID)
	}
	if !s.waitIdle(killWait) {
		slog.Warn("services: still running after the kill", "count", len(s.Running()))
	}
}

// killWait bounds the wait for a killed service: ptyHandle.Close sends SIGTERM,
// then SIGKILL after its own grace, and that is all it takes.
const killWait = 5 * time.Second

func (s *Service) waitIdle(limit time.Duration) bool {
	deadline := time.After(limit)
	for {
		s.mu.Lock()
		busy := false
		for _, r := range s.runs {
			if r.state == api.ServiceRunning || r.state == api.ServiceStopping {
				busy = true
				break
			}
		}
		changed := s.changed
		s.mu.Unlock()
		if !busy {
			return true
		}
		select {
		case <-changed:
		case <-deadline:
			return false
		}
	}
}
