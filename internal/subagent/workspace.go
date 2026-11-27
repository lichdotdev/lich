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
	// CardsOff is whether the user turned a provider's cards off (Cards.On);
	// nil turns none off.
	CardsOff func(provider string) bool
}

// Asker is session id's provider, label, subagent depth and whether its
// provider's cards are off.
func (w Workspace) Asker(id string) (Asker, bool) {
	projects, err := w.Load(context.Background(), api.None{})
	if err != nil {
		return Asker{}, false
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return Asker{Kind: sess.Kind, Label: sess.Label, Depth: w.Depth(id), CardsOff: w.CardsOff != nil && w.CardsOff(sess.Kind)}, true
			}
		}
	}
	return Asker{}, false
}
