package backend

import "context"

// RecentProject mirrors store.Recent: a closed project the reopen menu
// offers, and project.Project, what the picker and Relocate answer.
type RecentProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// CloseProject marks a project closed, keeping its sessions for a reopen.
func (c *Client) CloseProject(ctx context.Context, id string) error {
	return c.Call(ctx, "store.CloseProject", nil, id)
}

// ReorderProjects stores the open projects' order; ids names every one.
func (c *Client) ReorderProjects(ctx context.Context, ids []string) error {
	return c.Call(ctx, "store.ReorderProjects", nil, ids)
}

// RecentProjects returns the closed projects, the last one closed first.
func (c *Client) RecentProjects(ctx context.Context) ([]RecentProject, error) {
	var rows []RecentProject
	return rows, c.Call(ctx, "store.RecentProjects", &rows, "")
}

// ClosedProjectCount counts every closed project, past RecentProjects' page.
func (c *Client) ClosedProjectCount(ctx context.Context) (int, error) {
	var n int
	return n, c.Call(ctx, "store.ClosedProjectCount", &n, "")
}

// PickProject shows the backend's directory picker, nil when it is cancelled.
func (c *Client) PickProject(ctx context.Context) (*RecentProject, error) {
	var p *RecentProject
	return p, c.Call(ctx, "project.Open", &p)
}

// RelocateProject asks the picker where project id went, keeping its id; nil
// when cancelled. A directory another project holds is an error.
func (c *Client) RelocateProject(ctx context.Context, id string) (*RecentProject, error) {
	var p *RecentProject
	return p, c.Call(ctx, "project.Relocate", &p, id)
}

// PathExists reports whether a project directory is still there.
func (c *Client) PathExists(ctx context.Context, path string) (bool, error) {
	var ok bool
	return ok, c.Call(ctx, "project.Exists", &ok, path)
}

// MissingPaths returns the paths among paths that are gone.
func (c *Client) MissingPaths(ctx context.Context, paths []string) ([]string, error) {
	var gone []string
	return gone, c.Call(ctx, "project.Missing", &gone, paths)
}
