package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type Service = api.Service

// Services is every project service with its state.
func (c *Client) Services(ctx context.Context) ([]Service, error) {
	return callMethod(ctx, c, api.ServicesList, api.None{})
}

// SaveService writes a service, a new one when opts.ID is "", and returns it
// as stored.
func (c *Client) SaveService(ctx context.Context, opts api.SaveServiceOptions) (Service, error) {
	return callMethod(ctx, c, api.ServicesSave, opts)
}

// DeleteService removes a service; the backend refuses one that is running.
func (c *Client) DeleteService(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ServicesDelete, api.ServiceIDOptions{ID: id})
	return err
}

// StartService runs a service. One already running is left alone.
func (c *Client) StartService(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ServicesStart, api.ServiceIDOptions{ID: id})
	return err
}

// StopService asks a service to stop the way Ctrl+C does.
func (c *Client) StopService(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ServicesStop, api.ServiceIDOptions{ID: id})
	return err
}

// ForceStopService kills a service without waiting for it.
func (c *Client) ForceStopService(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ServicesForceStop, api.ServiceIDOptions{ID: id})
	return err
}

// RestartService stops a service and starts it again once it exited.
func (c *Client) RestartService(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.ServicesRestart, api.ServiceIDOptions{ID: id})
	return err
}

// StartAllServices starts every service of a project that is not running.
func (c *Client) StartAllServices(ctx context.Context, projectID string) error {
	_, err := callMethod(ctx, c, api.ServicesStartAll, api.ProjectIDOptions{ID: projectID})
	return err
}

// StopAllServices stops every running service of a project.
func (c *Client) StopAllServices(ctx context.Context, projectID string) error {
	_, err := callMethod(ctx, c, api.ServicesStopAll, api.ProjectIDOptions{ID: projectID})
	return err
}

// ServiceOutput is what a stopped service printed last, nil while it runs.
func (c *Client) ServiceOutput(ctx context.Context, id string) ([]byte, error) {
	return callMethod(ctx, c, api.ServicesOutput, api.ServiceIDOptions{ID: id})
}
