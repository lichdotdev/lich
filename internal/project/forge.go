package project

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// Forge answers which forge opts.Path's remote points at.
func (s *Service) Forge(_ context.Context, _ api.PathOptions) (string, error) {
	return api.ForgeGitHub, nil
}
