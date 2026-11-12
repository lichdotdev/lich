package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
)

// serviceAct is one of the service services' actions, keyed by service id.
type serviceAct = func(context.Context, api.ServiceIDOptions) (api.None, error)

// projectServices is the services of project, resolved as the service tools
// resolve it from outside a session.
func (s *Service) projectServices(ctx context.Context, project string) (api.ProjectRef, []api.Service, error) {
	ref, err := s.lich.Project(ctx, api.ProjectRefOptions{Project: project})
	if err != nil {
		return api.ProjectRef{}, nil, err
	}
	all, err := s.lich.Services(ctx, api.None{})
	if err != nil {
		return api.ProjectRef{}, nil, err
	}
	own := []api.Service{}
	for _, svc := range all {
		if svc.ProjectID == ref.ID {
			own = append(own, svc)
		}
	}
	return ref, own, nil
}

func (s *Service) findService(ctx context.Context, params extension.ServiceParams) (api.Service, error) {
	ref, list, err := s.projectServices(ctx, params.Project)
	if err != nil {
		return api.Service{}, err
	}
	names := make([]string, 0, len(list))
	for _, svc := range list {
		if strings.EqualFold(svc.Name, strings.TrimSpace(params.Service)) {
			return svc, nil
		}
		names = append(names, fmt.Sprintf("%q", svc.Name))
	}
	if len(names) == 0 {
		return api.Service{}, fmt.Errorf("%s has no services", ref.Name)
	}
	return api.Service{}, fmt.Errorf("no service named %q in %s. Its services: %s", params.Service, ref.Name, strings.Join(names, ", "))
}

func (s *Service) listServices(ctx context.Context, _ *installed, params extension.ServicesParams) ([]extension.Service, error) {
	_, list, err := s.projectServices(ctx, params.Project)
	if err != nil {
		return nil, err
	}
	out := make([]extension.Service, 0, len(list))
	for _, svc := range list {
		out = append(out, serviceOf(svc))
	}
	return out, nil
}

// serviceAction runs the action pick chooses and answers with the service as
// it is after, as the MCP service tools do. pick reads Lich at call time,
// since the routes are built before SetLich.
func (s *Service) serviceAction(pick func(Lich) serviceAct) func(context.Context, *installed, extension.ServiceParams) (extension.Service, error) {
	return func(ctx context.Context, _ *installed, params extension.ServiceParams) (extension.Service, error) {
		svc, err := s.findService(ctx, params)
		if err != nil {
			return extension.Service{}, err
		}
		if _, err := pick(s.lich)(ctx, api.ServiceIDOptions{ID: svc.ID}); err != nil {
			return extension.Service{}, err
		}
		after, err := s.findService(ctx, extension.ServiceParams{Project: params.Project, Service: svc.Name})
		if err != nil {
			return extension.Service{}, err
		}
		return serviceOf(after), nil
	}
}

// serviceOutput reads the live console of a running service and the last
// run's of any other, as read_service_output does.
func (s *Service) serviceOutput(ctx context.Context, _ *installed, params extension.ServiceParams) (extension.ServiceOutputResult, error) {
	svc, err := s.findService(ctx, params)
	if err != nil {
		return extension.ServiceOutputResult{}, err
	}
	read := s.lich.Output
	if svc.State == api.ServiceRunning || svc.State == api.ServiceStopping {
		read = s.lich.LiveOutput
	}
	out, err := read(ctx, api.ServiceIDOptions{ID: svc.ID})
	if err != nil {
		return extension.ServiceOutputResult{}, err
	}
	return extension.ServiceOutputResult{Output: string(out)}, nil
}

func serviceOf(svc api.Service) extension.Service {
	return extension.Service{
		ID: svc.ID, Name: svc.Name, Command: svc.Command, StartWithLich: svc.StartWithLich, State: svc.State,
		ExitCode: svc.ExitCode, StartedAt: svc.StartedAt, StoppedAt: svc.StoppedAt,
	}
}

// observeServices turns the services event, which carries every service, into
// one service.status per service whose state moved. A service seen for the
// first time moved when it is not stopped: a new one is saved stopped, and one
// that starts with lich is running in the first list. Called with mu held.
func (s *Service) observeServices(list []api.Service) {
	if s.serviceStates == nil {
		s.serviceStates = map[string]string{}
	}
	seen := make(map[string]string, len(list))
	for _, svc := range list {
		seen[svc.ID] = svc.State
		was, known := s.serviceStates[svc.ID]
		if was == svc.State || (!known && svc.State == api.ServiceStopped) {
			continue
		}
		s.broadcast(extension.ServiceStatus.Name, extension.ServiceStatus.New(extension.ServiceStatusData{
			ID: svc.ID, Name: svc.Name, ProjectID: svc.ProjectID, State: svc.State, ExitCode: svc.ExitCode,
		}))
	}
	s.serviceStates = seen
}

// broadcast queues msg for every extension subscribed to name. Called with mu
// held.
func (s *Service) broadcast(name string, msg extension.Message) {
	line, err := json.Marshal(msg)
	if err != nil {
		slog.Warn("extensions: encode an event", "event", name, "err", err)
		return
	}
	line = append(line, '\n')
	for _, x := range s.all {
		if x.subscribed[name] {
			s.enqueue(x, line)
		}
	}
}
