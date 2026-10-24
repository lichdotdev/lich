package services

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// List is every service with its state, each project's in the order they were
// added.
func (s *Service) List(_ context.Context, _ api.None) ([]api.Service, error) {
	return s.list()
}

// Start runs a service in its project's directory; see start.
func (s *Service) Start(_ context.Context, opts api.ServiceIDOptions) (api.None, error) {
	return api.None{}, s.start(opts.ID)
}

// Stop asks a service to stop the way Ctrl+C at its console does; see stop.
func (s *Service) Stop(_ context.Context, opts api.ServiceIDOptions) (api.None, error) {
	return api.None{}, s.stop(opts.ID)
}

// ForceStop kills a service without waiting for it to agree.
func (s *Service) ForceStop(_ context.Context, opts api.ServiceIDOptions) (api.None, error) {
	return api.None{}, s.forceStop(opts.ID)
}

// Restart stops a service and starts it again once it exited.
func (s *Service) Restart(_ context.Context, opts api.ServiceIDOptions) (api.None, error) {
	return api.None{}, s.restart(opts.ID)
}

// StartAll starts every service of a project that is not running.
func (s *Service) StartAll(_ context.Context, opts api.ProjectIDOptions) (api.None, error) {
	return api.None{}, s.startAll(opts.ID)
}

// StopAll stops every service of a project that is running.
func (s *Service) StopAll(_ context.Context, opts api.ProjectIDOptions) (api.None, error) {
	return api.None{}, s.stopAll(opts.ID)
}

// Output is what a service printed last, from the moment its last run exited:
// nil while it runs, its console being live on /ws then, or before it ever ran.
func (s *Service) Output(_ context.Context, opts api.ServiceIDOptions) ([]byte, error) {
	return s.output(opts.ID), nil
}
