package quota

import (
	"errors"
	"os"
	"strings"

	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/providers"
)

// ProfilePlan is the Claude Code plan of one profile (internal/claudeprofile),
// read the way a session spawned in it would be: Personal from lich's own
// environment, which is the machine-wide reading Settings shows, and any other
// profile from its own directory with no token in front of it. Settings shows
// one per profile. It is cached like a session's reading, under the account it
// reads, for the reason Plans gives.
func (s *Service) ProfilePlan(profile string) (Plan, error) {
	if claudeprofile.IsPersonal(profile) {
		return s.machinePlan(providers.Claude)
	}
	dir, err := claudeprofile.ConfigDir(profile)
	if err != nil {
		return Plan{}, err
	}
	account := Account{Read: true, Env: envMap(claudeprofile.Env(os.Environ(), dir))}
	return s.accountPlan(providers.Claude, account, s.claudePlan), nil
}

// CodexProfilePlan is ProfilePlan for a Codex profile (internal/codexprofile):
// Personal is the machine's own reading, any other profile is read from its own
// CODEX_HOME with nothing in front of its login. Plan.Account names the login's
// email, which is how Settings tells two Codex profiles apart.
func (s *Service) CodexProfilePlan(profile string) (Plan, error) {
	if codexprofile.IsPersonal(profile) {
		return s.machinePlan(providers.Codex)
	}
	dir, err := codexprofile.ConfigDir(profile)
	if err != nil {
		return Plan{}, err
	}
	account := Account{Read: true, Env: envMap(codexprofile.Env(os.Environ(), dir))}
	return s.accountPlan(providers.Codex, account, s.codexPlan), nil
}

// machinePlan is the machine-wide reading of a provider, which shares
// Plans("")'s cache entry instead of asking upstream a second time under a key
// of its own.
func (s *Service) machinePlan(provider string) (Plan, error) {
	for _, p := range s.Plans("") {
		if p.Provider == provider {
			return p, nil
		}
	}
	return Plan{}, errors.New("quota: the machine reading has no " + provider + " plan")
}

// accountPlan reads one provider's plan under an account, cached like a
// session's reading under the account it reads.
func (s *Service) accountPlan(provider string, account Account, read func(Account) Plan) Plan {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := provider + "\x00" + cacheKey(account)
	if cached, ok := s.cache[key]; ok && s.now().Sub(cached.at) < cacheTTL {
		return cached.plans[0]
	}
	plans := []Plan{read(account)}
	at := s.now()
	pace(plans, at)
	s.cache[key] = reading{plans: plans, at: at}
	return plans[0]
}

func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		m[key] = value
	}
	return m
}
