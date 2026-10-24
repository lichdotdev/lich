package relay

import (
	"context"
	"log/slog"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/store"
)

// AgentCheckouts is every checkout a live provider session runs in, each once:
// the ones the pull request watch (internal/project) reads every tick, since a
// pull request opens and merges there with no window looking. A workspace that
// fails to load follows none until the next call.
func (s *Service) AgentCheckouts() []string {
	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		slog.Warn("relay: read sessions for the pull request watch", "err", err)
		return nil
	}
	return s.agentCheckouts(projects)
}

func (s *Service) agentCheckouts(projects []store.Project) []string {
	seen := map[string]bool{}
	var checkouts []string
	for _, p := range projects {
		for _, sess := range p.Sessions {
			c := sessionCwd(sess.Path, p.Path)
			if !seen[c] && s.runsAgent(sess.ID, sess.Kind) {
				seen[c] = true
				checkouts = append(checkouts, c)
			}
		}
	}
	return checkouts
}
