package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type RecentProject = api.RecentProject

// CloseProject marks a project closed, keeping its sessions for a reopen.
func (c *Client) CloseProject(ctx context.Context, id string) error {
	_, err := callMethod(ctx, c, api.StoreCloseProject, api.ProjectIDOptions{ID: id})
	return err
}

// ReorderProjects stores the open projects' order; ids names every one.
func (c *Client) ReorderProjects(ctx context.Context, ids []string) error {
	_, err := callMethod(ctx, c, api.StoreReorderProjects, api.ReorderProjectsOptions{IDs: ids})
	return err
}

// RecentProjects returns the closed projects, the last one closed first.
func (c *Client) RecentProjects(ctx context.Context) ([]RecentProject, error) {
	return callMethod(ctx, c, api.StoreRecentProjects, api.SearchTermOptions{})
}

// ClosedProjectCount counts every closed project, past RecentProjects' page.
func (c *Client) ClosedProjectCount(ctx context.Context) (int, error) {
	return callMethod(ctx, c, api.StoreClosedProjectCount, api.SearchTermOptions{})
}

// PickProject shows the backend's directory picker, nil when it is cancelled.
func (c *Client) PickProject(ctx context.Context) (*RecentProject, error) {
	p, err := callMethod(ctx, c, api.ProjectOpen, api.None{})
	return (*RecentProject)(p), err
}

// RelocateProject asks the picker where project id went, keeping its id; nil
// when cancelled. A directory another project holds is an error.
func (c *Client) RelocateProject(ctx context.Context, id string) (*RecentProject, error) {
	p, err := callMethod(ctx, c, api.ProjectRelocate, api.ProjectIDOptions{ID: id})
	return (*RecentProject)(p), err
}

// PathExists reports whether a project directory is still there.
func (c *Client) PathExists(ctx context.Context, path string) (bool, error) {
	return callMethod(ctx, c, api.ProjectExists, api.PathOptions{Path: path})
}

// MissingPaths returns the paths among paths that are gone.
func (c *Client) MissingPaths(ctx context.Context, paths []string) ([]string, error) {
	return callMethod(ctx, c, api.ProjectMissing, api.PathsOptions{Paths: paths})
}
