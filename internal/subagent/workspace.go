package subagent

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/store"
)

// Workspace is Sessions over lich's store.
type Workspace struct {
	Load  func(context.Context, api.None) ([]store.Project, error)
	Depth func(id string) int
}

// Asker is session id's provider, label and subagent depth.
func (w Workspace) Asker(id string) (Asker, bool) {
	projects, err := w.Load(context.Background(), api.None{})
	if err != nil {
		return Asker{}, false
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return Asker{Kind: sess.Kind, Label: sess.Label, Depth: w.Depth(id)}, true
			}
		}
	}
	return Asker{}, false
}
