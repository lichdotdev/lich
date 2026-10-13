package quota

import (
	"errors"
	"os"
	"strings"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/providers"
)

// ProfilePlan is the Claude Code plan of one profile (internal/claudeprofile),
// read the way a session spawned in it would be: Personal from lich's own
// environment, which is the machine-wide reading Settings shows, and any other
// profile from its own directory with no token in front of it. Settings shows
// one per profile. It is cached like a session's reading, under the account it
// reads, for the reason Plans gives.
func (s *Service) ProfilePlan(profile string) (Plan, error) {
	if claudeprofile.IsPersonal(profile) {
		// The machine-wide reading, so it shares Plans("")'s cache entry
		// instead of asking upstream a second time under a key of its own.
		for _, p := range s.Plans("") {
			if p.Provider == providers.Claude {
				return p, nil
			}
		}
		return Plan{}, errors.New("quota: the machine reading has no Claude Code plan")
	}
	dir, err := claudeprofile.ConfigDir(profile)
	if err != nil {
		return Plan{}, err
	}
	account := Account{Read: true, Env: envMap(claudeprofile.Env(os.Environ(), dir))}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := providers.Claude + "\x00" + cacheKey(account)
	if cached, ok := s.cache[key]; ok && s.now().Sub(cached.at) < cacheTTL {
		return cached.plans[0], nil
	}
	plans := []Plan{s.claudePlan(account)}
	at := s.now()
	pace(plans, at)
	s.cache[key] = reading{plans: plans, at: at}
	return plans[0], nil
}

func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		m[key] = value
	}
	return m
}
