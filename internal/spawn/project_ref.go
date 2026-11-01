package spawn

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// Project resolves an open project as every other spawn call does, for a
// caller that goes on to a method keyed by project id.
func (s *Service) Project(ctx context.Context, opts api.ProjectRefOptions) (api.ProjectRef, error) {
	projects, err := s.sessions.LoadState(ctx, api.None{})
	if err != nil {
		return api.ProjectRef{}, fmt.Errorf("read the workspace: %w", err)
	}
	p, err := resolveProject(projects, opts.From, opts.Project)
	if err != nil {
		return api.ProjectRef{}, err
	}
	return api.ProjectRef{ID: p.ID, Name: p.Name, Path: p.Path}, nil
}
