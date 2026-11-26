package agentplugin

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/codexhooks"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"
)

// The plugin needs nothing from the person: a pass (Ensure) unpacks the newest
// compatible release when it is newer than the one there, takes out what a
// lich of the past installed where sessions now load that release, and
// installs or updates the plugin in every provider that still takes an
// install, for each provider the person turned on whose CLI is there. A pass
// runs at start, every ensureEvery after, and on a spawn or a report that
// finds the plugin missing; only what it could not do reaches the window.

// ensureEvery is how often a pass looks for a newer release, the cadence of
// lich's own update check (internal/appupdate).
const ensureEvery = time.Hour

type Problem = api.PluginProblem

// SetEmit wires where a pass reports what it could not do
// (api.PluginProblemEvent). Startup wiring.
func (s *Service) SetEmit(emit func(name string, data any)) { s.emit = emit }

// Watch runs a pass now and every ensureEvery after; it never returns.
func (s *Service) Watch() {
	for {
		s.Ensure()
		time.Sleep(ensureEvery)
	}
}

// Problem answers what the last pass could not do over RPC.
func (s *Service) Problem(_ context.Context, _ api.None) (Problem, error) {
	return s.currentProblem(), nil
}

// Retry runs a pass now, after one already running, and answers once it ran,
// its outcome on api.PluginProblemEvent as every pass's is.
func (s *Service) Retry(_ context.Context, _ api.None) (api.None, error) {
	s.ensureMu.Lock()
	defer s.ensureMu.Unlock()
	s.setProblem(s.ensure())
	return api.None{}, nil
}

// EnsureFor runs a pass in the background when kind's session is about to
// start without the plugin, or the last pass failed; "" leaves the provider out
// of the question. A pass already running is not
// started again.
func (s *Service) EnsureFor(kind string) {
	go func() {
		if s.needsPass(kind) {
			s.Ensure()
		}
	}()
}

// needsPass is whether the last pass failed, or kind's provider is on and
// missing the plugin.
func (s *Service) needsPass(kind string) bool {
	return s.currentProblem().Error != "" || (kind != "" && s.wanted(kind) && !s.Installed(kind))
}

// Ensure is one pass, skipped while another runs.
func (s *Service) Ensure() {
	if !s.ensureMu.TryLock() {
		return
	}
	defer s.ensureMu.Unlock()
	s.setProblem(s.ensure())
}

func (s *Service) ensure() Problem {
	var p Problem
	fail := func(name string, err error) {
		if name != "" {
			p.Providers = append(p.Providers, name)
		}
		if p.Error == "" {
			p.Error = err.Error()
		}
	}
	latest := s.latestVersion()
	err := errNoRelease()
	if latest != "" {
		err = s.installRelease(latest)
	}
	_, unpacked := bundleVersion()
	switch {
	case !unpacked:
		p.Bundle = true
		fail("", err)
	case err != nil:
		slog.Warn("agentplugin: unpack the newest release", "err", err)
	}
	if unpacked {
		s.pruneReleases()
	}
	s.migrateInstalls()
	for _, id := range supported {
		if !s.wanted(id) || (loadsBundle(id) && !unpacked) {
			continue
		}
		if err := s.ensureIn(id, latest); err != nil {
			if _, installed := s.installedVersion(id); !installed {
				fail(providerName(id), err)
				continue
			}
			slog.Warn("agentplugin: update the plugin", "provider", id, "err", err)
		}
	}
	if s.wanted(providers.Codex) {
		s.trustCodexHooks()
	}
	return p
}

// ensureIn installs the plugin in provider when it is missing, behind latest
// or written for another release of the CLI, and brings Codex's profiles to
// Personal's release either way.
func (s *Service) ensureIn(provider, latest string) error {
	version, installed := s.installedVersion(provider)
	st := computeStatus(provider, true, installed, version, latest)
	behind := provider == providers.Crush && installed && s.crushBlockBehind()
	if installed && !st.UpdateAvailable && !behind {
		if provider == providers.Codex {
			return s.followPersonal(provider)
		}
		return nil
	}
	return s.install(provider)
}

// loadsBundle is whether provider's sessions load lich's unpacked release,
// so nothing can be installed for them without one.
func loadsBundle(provider string) bool {
	return slices.Contains([]string{providers.Claude, providers.Cursor, providers.OpenCode, providers.OMP}, provider)
}

// wanted is whether a pass sets the plugin up for provider: the person turned
// it on and its CLI is there.
func (s *Service) wanted(provider string) bool {
	on, err := s.bins.ProviderEnabled(provider)
	if err != nil {
		slog.Warn("agentplugin: read whether a provider is on", "provider", provider, "err", err)
		return false
	}
	return on && slices.Contains(supported, provider) && s.available(provider)
}

func (s *Service) currentProblem() Problem {
	s.problemMu.Lock()
	defer s.problemMu.Unlock()
	return s.problem
}

// setProblem files a pass's outcome and reports it when it changed: a failure,
// or the end of one.
func (s *Service) setProblem(p Problem) {
	s.problemMu.Lock()
	same := s.problem.Bundle == p.Bundle && s.problem.Error == p.Error && slices.Equal(s.problem.Providers, p.Providers)
	s.problem = p
	s.problemMu.Unlock()
	if same || s.emit == nil {
		return
	}
	events.Send(s.emit, api.PluginProblemEvent, p)
}

// trustCodexHooks trusts lich-plugin's hooks in Personal and every Codex
// profile (codexhooks.TrustLich), so Codex runs them with no /hooks step: it
// runs none it holds as untrusted or changed since trusted, and an install or
// update changes them.
func (s *Service) trustCodexHooks() {
	homes := []string{""}
	if profiles, err := codexprofile.List(); err == nil {
		for _, p := range profilesBesidePersonal(profiles) {
			homes = append(homes, p.Dir)
		}
	}
	cwd, _ := os.UserHomeDir()
	for _, home := range homes {
		var env []string
		if home != "" {
			env = codexprofile.Env(os.Environ(), home)
		}
		cmd := s.command(providers.Codex, env, codexAppServer)
		cmd.Dir = cwd
		if n, err := codexhooks.TrustLich(context.Background(), cmd, cwd); err != nil {
			slog.Warn("agentplugin: trust the Codex hooks", "home", home, "err", err)
		} else if n > 0 {
			slog.Info("agentplugin: trusted the Codex hooks", "home", home, "hooks", n)
		}
	}
}

// codexAppServer is the Codex subcommand trustCodexHooks talks to.
const codexAppServer = "app-server"

// command is provider's CLI with args, under env (nil for lich's own), not yet
// started.
func (s *Service) command(provider string, env []string, args ...string) *exec.Cmd {
	cmd := exec.Command(s.bin(provider), args...)
	cmd.Env = env
	winexec.Hide(cmd)
	return cmd
}
